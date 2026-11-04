package schedules

import (
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/semaphoreui/semaphore/db"
	"github.com/semaphoreui/semaphore/db/sql"
	"github.com/semaphoreui/semaphore/pkg/ssh"
	"github.com/semaphoreui/semaphore/pkg/task_logger"
	"github.com/semaphoreui/semaphore/services/tasks"
	"github.com/semaphoreui/semaphore/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEncryptionService is a test implementation of AccessKeyEncryptionService
type mockEncryptionService struct{}

func (m *mockEncryptionService) RekeyAccessKeys(oldKey string) (err error) {
	return nil
}

func (m *mockEncryptionService) SerializeSecret(key *db.AccessKey) error {
	return nil
}

func (m *mockEncryptionService) DeserializeSecret(key *db.AccessKey) error {
	return nil
}

func (m *mockEncryptionService) FillEnvironmentSecrets(env *db.Environment, deserializeSecret bool) error {
	return nil
}

func (m *mockEncryptionService) DeleteSecret(key *db.AccessKey) error {
	return nil
}

func (m *mockEncryptionService) CreateTaskSurveySecrets(projectID int, taskID int, secrets string, expireAt time.Time) error {
	return nil
}

func (m *mockEncryptionService) GetTaskSurveySecrets(projectID int, taskID int) (string, error) {
	return "", nil
}

func (m *mockEncryptionService) DeleteTaskSurveySecrets(projectID int, taskID int) error {
	return nil
}

func TestValidateCronFormat(t *testing.T) {
	err := ValidateCronFormat("* * * *")
	if err == nil {
		t.Fatal("")
	}

	err = ValidateCronFormat("* * 1 * *")
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestOneTimeSchedule(t *testing.T) {
	future := time.Now().Add(time.Hour)
	schedule := oneTimeSchedule{runAt: future}

	if schedule.Next(time.Now()) != future {
		t.Fatalf("expected next run at %v", future)
	}

	if !schedule.Next(future).IsZero() {
		t.Fatalf("expected schedule to stop after run time")
	}
}

func TestOnBootScheduleRunsOncePerProcessAndSurvivesRefresh(t *testing.T) {
	store := sql.InitConfigCreateTestStore()
	originalScheduleConfig := util.Config.Schedule
	util.Config.Schedule = &util.ScheduleConfig{Timezone: "UTC"}
	t.Cleanup(func() {
		util.Config.Schedule = originalScheduleConfig
	})
	project, err := store.CreateProject(db.Project{Name: "on-boot"})
	require.NoError(t, err)

	key, err := store.CreateAccessKey(db.AccessKey{ProjectID: &project.ID, Type: db.AccessKeyNone})
	require.NoError(t, err)
	repo, err := store.CreateRepository(db.Repository{
		ProjectID: project.ID,
		Name:      "repo",
		GitURL:    "https://example.com/repo.git",
		GitBranch: "main",
		SSHKeyID:  key.ID,
	})
	require.NoError(t, err)
	template, err := store.CreateTemplate(db.Template{
		ProjectID:    project.ID,
		RepositoryID: repo.ID,
		Name:         "template",
		Playbook:     "site.yml",
	})
	require.NoError(t, err)
	schedule, err := store.CreateSchedule(db.Schedule{
		ProjectID:     project.ID,
		TemplateID:    template.ID,
		Name:          "after startup",
		Type:          db.ScheduleTypeOnBoot,
		TimeAfterBoot: 90,
		Active:        true,
	})
	require.NoError(t, err)

	pool := CreateSchedulePool(store, &tasks.TaskPool{}, &mockAccessKeyInstaller{}, &mockEncryptionService{})
	t.Cleanup(pool.Destroy)
	bootTime, eligibleIDs := pool.GetOnBootState()
	assert.WithinDuration(t, time.Now(), bootTime, time.Second)
	assert.Equal(t, []int{schedule.ID}, eligibleIDs)
	assert.True(t, pool.IsOnBootScheduleEligible(schedule.ID))
	go pool.Run()

	bootRunAt := pool.bootTime.Add(90 * time.Second)
	var entries []cron.Entry
	require.Eventually(t, func() bool {
		entries = pool.cron.Entries()
		return len(entries) == 1 && !entries[0].Next.IsZero()
	}, time.Second, time.Millisecond*10)
	assert.Equal(t, bootRunAt, entries[0].Next)

	loaded, err := store.GetSchedule(project.ID, schedule.ID)
	require.NoError(t, err)
	assert.Equal(t, 90, loaded.TimeAfterBoot)

	pool.Refresh()
	require.Eventually(t, func() bool {
		entries = pool.cron.Entries()
		return len(entries) == 1 && !entries[0].Next.IsZero()
	}, time.Second, time.Millisecond*10)
	assert.Equal(t, bootRunAt, entries[0].Next)

	assert.True(t, pool.claimOnBootSchedule(schedule.ID))
	assert.False(t, pool.claimOnBootSchedule(schedule.ID))
	pool.Refresh()
	assert.Empty(t, pool.cron.Entries())

	lateSchedule, err := store.CreateSchedule(db.Schedule{
		ProjectID:     project.ID,
		TemplateID:    template.ID,
		Name:          "created after startup",
		Type:          db.ScheduleTypeOnBoot,
		TimeAfterBoot: 0,
		Active:        true,
	})
	require.NoError(t, err)
	pool.Refresh()
	assert.NotContains(t, pool.onBootScheduleIDs, lateSchedule.ID)
	assert.Empty(t, pool.cron.Entries())
}

// mockDeduplicator is a test implementation of ScheduleDeduplicator
type mockDeduplicator struct {
	mu             sync.Mutex
	allowExecution map[int]bool
	lockAttempts   map[int]int
}

func newMockDeduplicator() *mockDeduplicator {
	return &mockDeduplicator{
		allowExecution: make(map[int]bool),
		lockAttempts:   make(map[int]int),
	}
}

func (m *mockDeduplicator) TryLockExecution(scheduleID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lockAttempts[scheduleID]++
	return m.allowExecution[scheduleID]
}

func (m *mockDeduplicator) setAllowExecution(scheduleID int, allow bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowExecution[scheduleID] = allow
}

func (m *mockDeduplicator) getLockAttempts(scheduleID int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lockAttempts[scheduleID]
}

// mockAccessKeyInstaller is a test implementation of AccessKeyInstaller
type mockAccessKeyInstaller struct{}

func (m *mockAccessKeyInstaller) Install(key db.AccessKey, usage db.AccessKeyRole, logger task_logger.Logger) (installation ssh.AccessKeyInstallation, err error) {
	return ssh.AccessKeyInstallation{}, nil
}

func setupTestSchedulePool(t *testing.T) (*SchedulePool, db.Store) {
	store := sql.InitConfigCreateTestStore()

	// Store original config and restore after test
	originalSchedule := util.Config.Schedule
	t.Cleanup(func() {
		util.Config.Schedule = originalSchedule
	})

	// Ensure util.Config Schedule is set (CreateTestStore doesn't set this)
	util.Config.Schedule = &util.ScheduleConfig{
		Timezone: "UTC",
	}

	pool := CreateSchedulePool(
		store,
		&tasks.TaskPool{},
		&mockAccessKeyInstaller{},
		&mockEncryptionService{},
	)

	t.Cleanup(func() {
		pool.Destroy()
	})

	return &pool, store
}

// TestSetDeduplicator verifies that SetDeduplicator properly configures the deduplicator
func TestSetDeduplicator(t *testing.T) {
	pool, _ := setupTestSchedulePool(t)

	// Initially no deduplicator should be set
	assert.Nil(t, pool.dedup, "deduplicator should be nil initially")

	// Set a deduplicator
	dedup := newMockDeduplicator()
	pool.SetDeduplicator(dedup)

	// Verify it's set
	assert.NotNil(t, pool.dedup, "deduplicator should be set after calling SetDeduplicator")
	assert.Equal(t, dedup, pool.dedup, "deduplicator should be the one we set")
}

// TestScheduleExecutesNormallyWithoutDeduplicator verifies schedules execute when no deduplicator is set
func TestScheduleExecutesNormallyWithoutDeduplicator(t *testing.T) {
	pool, _ := setupTestSchedulePool(t)

	// Ensure no deduplicator is set
	pool.SetDeduplicator(nil)

	// Verify that the deduplicator is nil (schedule would execute normally)
	assert.Nil(t, pool.dedup, "deduplicator should be nil, allowing normal execution")
}

// TestScheduleSkippedWhenTryLockExecutionReturnsFalse verifies schedules are skipped when TryLockExecution returns false
func TestScheduleSkippedWhenTryLockExecutionReturnsFalse(t *testing.T) {
	pool, _ := setupTestSchedulePool(t)

	// Set up deduplicator to deny execution
	dedup := newMockDeduplicator()
	scheduleID := 123
	dedup.setAllowExecution(scheduleID, false)
	pool.SetDeduplicator(dedup)

	// Simulate the deduplication check that happens in ScheduleRunner.Run()
	shouldSkip := pool.dedup != nil && !pool.dedup.TryLockExecution(scheduleID)

	// Verify the deduplicator was called and returned false
	assert.True(t, shouldSkip, "schedule should be skipped when TryLockExecution returns false")
	assert.Equal(t, 1, dedup.getLockAttempts(scheduleID), "TryLockExecution should be called once")
}

// TestScheduleProceedsWhenTryLockExecutionReturnsTrue verifies schedules proceed when TryLockExecution returns true
func TestScheduleProceedsWhenTryLockExecutionReturnsTrue(t *testing.T) {
	pool, _ := setupTestSchedulePool(t)

	// Set up deduplicator to allow execution
	dedup := newMockDeduplicator()
	scheduleID := 456
	dedup.setAllowExecution(scheduleID, true)
	pool.SetDeduplicator(dedup)

	// Simulate the deduplication check that happens in ScheduleRunner.Run()
	shouldSkip := pool.dedup != nil && !pool.dedup.TryLockExecution(scheduleID)

	// Verify the deduplicator was called and returned true (schedule proceeds)
	assert.False(t, shouldSkip, "schedule should proceed when TryLockExecution returns true")
	assert.Equal(t, 1, dedup.getLockAttempts(scheduleID), "TryLockExecution should be called once")
}
