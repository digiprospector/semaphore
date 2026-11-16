package sql

import (
	"testing"
	"time"

	"github.com/semaphoreui/semaphore/db"
	"github.com/semaphoreui/semaphore/pkg/task_logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkRunningTasksAsSuccess(t *testing.T) {
	store := InitConfigCreateTestStore()
	t.Cleanup(store.Close)

	projectID, repositoryID := newTemplateTestProject(t, store)
	template, err := store.CreateTemplate(db.Template{
		ProjectID:    projectID,
		RepositoryID: repositoryID,
		Name:         "tpl",
		Playbook:     "site.yml",
	})
	require.NoError(t, err)

	createTask := func(status task_logger.TaskStatus, runnerID *int) db.Task {
		task, err := store.CreateTask(db.Task{
			ProjectID:  projectID,
			TemplateID: template.ID,
			Status:     status,
			Playbook:   "site.yml",
			Created:    time.Now(),
			RunnerID:   runnerID,
		}, 0)
		require.NoError(t, err)
		return task
	}

	// Local running task (no runner) -- the orphan this fix targets.
	localRunning := createTask(task_logger.TaskRunningStatus, nil)
	// Running task dispatched to a remote runner -- must be preserved.
	runner, err := store.CreateRunner(db.Runner{Name: "r1", Token: "runner-token"})
	require.NoError(t, err)
	remoteRunning := createTask(task_logger.TaskRunningStatus, &runner.ID)
	// Excluded task -- still tracked by the live cluster, must be preserved.
	excluded := createTask(task_logger.TaskRunningStatus, nil)
	// Terminal task -- untouched.
	successTask := createTask(task_logger.TaskSuccessStatus, nil)

	count, err := store.MarkRunningTasksAsSuccess([]int{excluded.ID})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	after, err := store.GetTaskByID(localRunning.ID)
	require.NoError(t, err)
	assert.Equal(t, task_logger.TaskSuccessStatus, after.Status)
	require.NotNil(t, after.End)

	remoteAfter, err := store.GetTaskByID(remoteRunning.ID)
	require.NoError(t, err)
	assert.Equal(t, task_logger.TaskRunningStatus, remoteAfter.Status)

	excludedAfter, err := store.GetTaskByID(excluded.ID)
	require.NoError(t, err)
	assert.Equal(t, task_logger.TaskRunningStatus, excludedAfter.Status)

	successAfter, err := store.GetTaskByID(successTask.ID)
	require.NoError(t, err)
	assert.Equal(t, task_logger.TaskSuccessStatus, successAfter.Status)
}

func TestMarkRunningTasksAsSuccess_NoExclusions(t *testing.T) {
	store := InitConfigCreateTestStore()
	t.Cleanup(store.Close)

	projectID, repositoryID := newTemplateTestProject(t, store)
	template, err := store.CreateTemplate(db.Template{
		ProjectID:    projectID,
		RepositoryID: repositoryID,
		Name:         "tpl",
		Playbook:     "site.yml",
	})
	require.NoError(t, err)

	localRunning, err := store.CreateTask(db.Task{
		ProjectID:  projectID,
		TemplateID: template.ID,
		Status:     task_logger.TaskRunningStatus,
		Playbook:   "site.yml",
		Created:    time.Now(),
	}, 0)
	require.NoError(t, err)

	count, err := store.MarkRunningTasksAsSuccess(nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	after, err := store.GetTaskByID(localRunning.ID)
	require.NoError(t, err)
	assert.Equal(t, task_logger.TaskSuccessStatus, after.Status)
}
