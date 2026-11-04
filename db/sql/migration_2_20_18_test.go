package sql

import (
	"testing"

	"github.com/semaphoreui/semaphore/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration_2_20_18_GivesReadToRolesAndGrants(t *testing.T) {
	before := "2.20.17"
	store := InitConfigCreateTestStoreAt(&before)
	projectID, repositoryID := newTemplateTestProject(t, store)
	template, err := store.CreateTemplate(db.Template{ProjectID: projectID, RepositoryID: repositoryID, Name: "deploy", Playbook: "site.yml"})
	require.NoError(t, err)
	workflowID := insertWorkflow(t, store, projectID, "release")
	ops, err := store.CreateRole(db.Role{Slug: "ops", Name: "Ops", Permissions: 1 | 16 | 64, ProjectID: &projectID})
	require.NoError(t, err)
	nobody, err := store.CreateRole(db.Role{Slug: "nobody", Name: "Nobody", ProjectID: &projectID})
	require.NoError(t, err)
	templateGrant, err := store.CreateTemplateRole(db.TemplateRolePerm{ProjectID: projectID, TemplateID: template.ID, RoleID: nobody.ID, Permissions: 1})
	require.NoError(t, err)
	workflowGrant, err := store.CreateWorkflowRole(db.WorkflowRolePerm{ProjectID: projectID, WorkflowID: workflowID, RoleID: nobody.ID, Permissions: 16})
	require.NoError(t, err)
	emptyGrant, err := store.CreateTemplateRole(db.TemplateRolePerm{ProjectID: projectID, TemplateID: template.ID, RoleID: ops.ID, Permissions: 0})
	require.NoError(t, err)

	target := "2.20.18"
	require.NoError(t, db.Migrate(store, &target))

	for slug, mask := range map[string]db.ProjectUserPermission{"owner": 255, "guest": 128, "ops": 1 | 16 | 64 | 128, "nobody": 128} {
		role, err := store.GetProjectOrSharedRoleBySlug(projectID, slug)
		require.NoError(t, err)
		assert.Equal(t, mask, role.Permissions, slug)
	}
	storedTemplateGrant, err := store.GetTemplateRole(projectID, template.ID, templateGrant.ID)
	require.NoError(t, err)
	assert.Equal(t, db.ProjectUserPermission(1|128), storedTemplateGrant.Permissions)
	storedWorkflowGrant, err := store.GetWorkflowRole(projectID, workflowID, workflowGrant.ID)
	require.NoError(t, err)
	assert.Equal(t, db.ProjectUserPermission(16|128), storedWorkflowGrant.Permissions)
	storedEmptyGrant, err := store.GetTemplateRole(projectID, template.ID, emptyGrant.ID)
	require.NoError(t, err)
	assert.Equal(t, db.ProjectUserPermission(0), storedEmptyGrant.Permissions, "a grant without a verb stays empty")

	require.NoError(t, db.Rollback(store, "2.20.17"))
	role, err := store.GetProjectOrSharedRoleBySlug(projectID, "ops")
	require.NoError(t, err)
	assert.Equal(t, ops.Permissions, role.Permissions, "rollback keeps the old bits only")
	storedTemplateGrant, err = store.GetTemplateRole(projectID, template.ID, templateGrant.ID)
	require.NoError(t, err)
	assert.Equal(t, db.ProjectUserPermission(1), storedTemplateGrant.Permissions)
}

func TestMigration_2_20_18_RepairsLegacy217WithoutWorkflowRole(t *testing.T) {
	before := "2.20.17"
	store := InitConfigCreateTestStoreAt(&before)
	target := "2.20.18"

	_, err := store.exec("update `role` set `permissions`=5 where slug='owner'")
	require.NoError(t, err)
	_, err = store.exec("drop table `project__workflow_role`")
	require.NoError(t, err)

	require.NoError(t, db.Migrate(store, &target))

	owner, err := store.GetSharedRoleBySlug("owner")
	require.NoError(t, err)
	assert.Equal(t, db.ProjectUserPermission(245), owner.Permissions)
	tableCount, err := store.Sql().SelectInt(store.PrepareQuery(
		"select count(*) from sqlite_master where type='table' and name='project__workflow_role'"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), tableCount)
	indexCount, err := store.Sql().SelectInt(store.PrepareQuery(
		"select count(*) from sqlite_master where type='index' and name in ('project__workflow_role__role_id', 'project__workflow_role__project_id')"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), indexCount)
}
