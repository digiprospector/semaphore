package sql

import (
	"testing"
	"time"

	"github.com/semaphoreui/semaphore/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetScheduleTasks(t *testing.T) {
	store := InitConfigCreateTestStore()
	t.Cleanup(store.Close)

	projectID, repositoryID := newTemplateTestProject(t, store)
	template, err := store.CreateTemplate(db.Template{
		ProjectID:    projectID,
		RepositoryID: repositoryID,
		Name:         "boot template",
		Playbook:     "site.yml",
	})
	require.NoError(t, err)
	schedule, err := store.CreateSchedule(db.Schedule{
		ProjectID:  projectID,
		TemplateID: template.ID,
		Name:       "boot schedule",
		Type:       db.ScheduleTypeOnBoot,
		Active:     true,
	})
	require.NoError(t, err)

	createdTask, err := store.CreateTask(db.Task{
		ProjectID:  projectID,
		TemplateID: template.ID,
		ScheduleID: &schedule.ID,
		Status:     "success",
		Playbook:   "site.yml",
		Created:    time.Now(),
	}, 0)
	require.NoError(t, err)
	_, err = store.CreateTask(db.Task{
		ProjectID:  projectID,
		TemplateID: template.ID,
		Status:     "success",
		Playbook:   "site.yml",
		Created:    time.Now(),
	}, 0)
	require.NoError(t, err)
	cronSchedule, err := store.CreateSchedule(db.Schedule{
		ProjectID:  projectID,
		TemplateID: template.ID,
		Name:       "cron schedule",
		CronFormat: "0 0 * * *",
		Active:     true,
	})
	require.NoError(t, err)
	_, err = store.CreateTask(db.Task{
		ProjectID:  projectID,
		TemplateID: template.ID,
		ScheduleID: &cronSchedule.ID,
		Status:     "success",
		Playbook:   "site.yml",
		Created:    time.Now(),
	}, 0)
	require.NoError(t, err)

	tasks, err := store.GetScheduleTasks(projectID, []int{schedule.ID}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, createdTask.ID, tasks[0].ID)
	assert.Equal(t, schedule.ID, *tasks[0].ScheduleID)
	assert.Equal(t, "boot template", tasks[0].TemplateAlias)

	tasks, err = store.GetScheduleTasks(projectID, []int{}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	assert.Empty(t, tasks)

	tasks, err = store.GetOnBootScheduleTasks(projectID, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, createdTask.ID, tasks[0].ID)
}
