package sql

import (
	"github.com/go-gorp/gorp/v3"
	"github.com/semaphoreui/semaphore/util"
)

type migration_2_20_18 struct {
	db *SqlDb
}

func (m migration_2_20_18) PreApply(tx *gorp.Transaction) error {
	queries := []string{
		"update `role` set `permissions` = `permissions` | 16 | 64 where `permissions` & 1 <> 0",
		"update `role` set `permissions` = `permissions` | 32 where `permissions` & 4 <> 0",
	}
	for _, query := range queries {
		if _, err := tx.Exec(m.db.PrepareQuery(query)); err != nil {
			return err
		}
	}

	tableQuery := "select count(*) from sqlite_master where type='table' and name=?"
	switch m.db.GetDialect() {
	case util.DbDriverMySQL:
		tableQuery = "select count(*) from information_schema.tables where table_schema=database() and table_name=?"
	case "postgres":
		tableQuery = "select count(*) from information_schema.tables where table_schema=current_schema() and table_name=?"
	}

	tableCount, err := tx.SelectInt(m.db.PrepareQuery(tableQuery), "project__workflow_role")
	if err != nil {
		return err
	}
	if tableCount > 0 {
		return nil
	}

	statements := []string{
		"create table `project__workflow_role` (" +
			"`id` integer primary key autoincrement, " +
			"`workflow_id` int not null, " +
			"`role_id` int not null, " +
			"`project_id` int not null, " +
			"`permissions` bigint not null default 0, " +
			"foreign key (`workflow_id`) references `project__workflow_template` (`id`) on delete cascade, " +
			"foreign key (`role_id`) references `role` (`id`) on delete restrict, " +
			"foreign key (`project_id`) references `project` (`id`) on delete cascade, " +
			"unique (`workflow_id`, `role_id`))",
		"create index `project__workflow_role__role_id` on `project__workflow_role` (`role_id`)",
		"create index `project__workflow_role__project_id` on `project__workflow_role` (`project_id`)",
	}
	for _, statement := range statements {
		if _, err = tx.Exec(m.db.prepareMigration(statement)); err != nil {
			return err
		}
	}

	return nil
}
