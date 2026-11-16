package tasks

import (
	"testing"
	"time"

	"github.com/semaphoreui/semaphore/db"
	"github.com/semaphoreui/semaphore/db/sql"
	"github.com/semaphoreui/semaphore/pkg/task_logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPool_RecoverStaleRunningTasks(t *testing.T) {
	store := sql.InitConfigCreateTestStore()
	t.Cleanup(store.Close)

	project, err := store.CreateProject(db.Project{Name: "proj"})
	require.NoError(t, err)

	key, err := store.CreateAccessKey(db.AccessKey{
		ProjectID: &project.ID,
		Type:      db.AccessKeyNone,
	})
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
		Name:         "tpl",
		Playbook:     "site.yml",
	})
	require.NoError(t, err)

	createRunningTask := func() db.Task {
		task, err := store.CreateTask(db.Task{
			ProjectID:  project.ID,
			TemplateID: template.ID,
			Status:     task_logger.TaskRunningStatus,
			Playbook:   "site.yml",
			Created:    time.Now(),
		}, 0)
		require.NoError(t, err)
		return task
	}

	stale := createRunningTask()
	tracked := createRunningTask()

	state := NewMemoryTaskStateStore()
	state.SetRunning(&TaskRunner{Task: tracked})

	pool := TaskPool{state: state, store: store}
	pool.recoverStaleRunningTasks()

	staleAfter, err := store.GetTaskByID(stale.ID)
	require.NoError(t, err)
	assert.Equal(t, task_logger.TaskSuccessStatus, staleAfter.Status)
	assert.NotNil(t, staleAfter.End)

	trackedAfter, err := store.GetTaskByID(tracked.ID)
	require.NoError(t, err)
	assert.Equal(t, task_logger.TaskRunningStatus, trackedAfter.Status)
}
