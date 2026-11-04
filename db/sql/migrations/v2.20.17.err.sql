alter table `project__schedule` drop column `time_after_boot`;

drop table `project__workflow_role`;

update `role` set `permissions` = `permissions` & 15;
