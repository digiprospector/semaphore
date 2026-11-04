import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc';
import timezone from 'dayjs/plugin/timezone';
import { CronExpressionParser } from 'cron-parser';

dayjs.extend(utc);
dayjs.extend(timezone);

function getTasksTodayBySchedule(tasks, dayStart, nextDay) {
  return tasks.reduce((result, task) => {
    if (task.schedule_id == null) return result;

    const taskStart = dayjs(task.start || task.created);
    const startedToday = !taskStart.isBefore(dayStart) && taskStart.isBefore(nextDay);
    const stillRunningToday = task.start && !task.end && taskStart.isBefore(nextDay);
    if (!startedToday && !stillRunningToday) return result;

    const scheduleID = Number(task.schedule_id);
    result.set(scheduleID, [...(result.get(scheduleID) || []), task]);
    return result;
  }, new Map());
}

export function getTodayScheduleEntries(schedules, timezoneName, now = new Date(), tasks = []) {
  const currentTime = dayjs(now);
  const dayStart = currentTime.tz(timezoneName).startOf('day');
  const nextDay = dayStart.add(1, 'day');
  const tasksToday = getTasksTodayBySchedule(tasks, dayStart, nextDay);

  const entries = [];
  schedules.forEach((schedule) => {
    const scheduleTasks = tasksToday.get(Number(schedule.id)) || [];
    const occurrences = [];

    if (schedule.type === 'run_at') {
      const runAt = schedule.run_at ? dayjs(schedule.run_at) : null;
      if (runAt && runAt.isValid() && !runAt.isBefore(dayStart) && runAt.isBefore(nextDay)) {
        occurrences.push(runAt);
      }
    } else if ((schedule.type || '') === '' && schedule.cron_format) {
      try {
        const iterator = CronExpressionParser.parse(schedule.cron_format, {
          currentDate: dayStart.subtract(1, 'minute').toDate(),
          tz: timezoneName,
        });
        let occurrence = dayjs(iterator.next().toDate());
        while (!occurrence.isBefore(dayStart) && occurrence.isBefore(nextDay)) {
          occurrences.push(occurrence);
          occurrence = dayjs(iterator.next().toDate());
        }
      } catch {
        // Ignore invalid cron expressions, as the schedule editor reports them separately.
      }
    }

    const matchedTasks = scheduleTasks.filter((task) => {
      const taskTime = dayjs(task.created || task.start);
      return occurrences.some((occurrence) => (
        Math.abs(taskTime.diff(occurrence)) < 60 * 1000
      ));
    });
    matchedTasks.forEach((task) => {
      const taskTime = dayjs(task.created || task.start);
      const occurrence = occurrences.find((candidate) => (
        Math.abs(taskTime.diff(candidate)) < 60 * 1000
      ));
      entries.push({
        ...schedule,
        row_key: `task-${task.id}`,
        first_run_at: occurrence.toISOString(),
        task,
      });
    });

    const matchedTaskIDs = new Set(matchedTasks.map((task) => task.id));
    occurrences.forEach((occurrence) => {
      const hasTask = matchedTasks.some((task) => {
        const taskTime = dayjs(task.created || task.start);
        return Math.abs(taskTime.diff(occurrence)) < 60 * 1000;
      });
      if (!hasTask && schedule.active && !occurrence.isBefore(currentTime)) {
        entries.push({
          ...schedule,
          row_key: `schedule-${schedule.id}-${occurrence.valueOf()}`,
          first_run_at: occurrence.toISOString(),
          task: null,
        });
      }
    });

    scheduleTasks.forEach((task) => {
      if (matchedTaskIDs.has(task.id)) return;
      entries.push({
        ...schedule,
        row_key: `task-${task.id}`,
        first_run_at: dayjs(task.created || task.start).toISOString(),
        task,
      });
    });
  });

  return entries.sort((a, b) => new Date(a.first_run_at) - new Date(b.first_run_at));
}

export function getBootScheduleEntries(schedules, tasks, bootTime, eligibleScheduleIDs) {
  if (!bootTime) return [];

  const bootStart = dayjs(bootTime);
  const bootSchedules = schedules.filter((schedule) => schedule.type === 'on_boot');
  const schedulesByID = new Map(bootSchedules.map((schedule) => [Number(schedule.id), schedule]));
  const eligibleIDs = new Set(eligibleScheduleIDs.map(Number));
  const handledSinceBoot = new Set();
  const rows = [];

  tasks.forEach((task) => {
    const scheduleID = Number(task.schedule_id);
    if (task.schedule_id == null) return;
    const schedule = schedulesByID.get(scheduleID) || {
      id: scheduleID,
      project_id: task.project_id,
      template_id: task.template_id,
      tpl_name: task.tpl_alias,
      name: '',
      active: false,
      type: 'on_boot',
      time_after_boot: 0,
    };

    const taskEnd = task.end ? dayjs(task.end) : null;
    const finished = taskEnd || ['success', 'error', 'stopped'].includes(task.status);
    let section = '4_pending';
    if (finished) {
      const finishedAt = taskEnd || dayjs(task.start || task.created);
      section = finishedAt.isBefore(bootStart) ? '1_history' : '2_completed';
    } else if (task.start) {
      section = '3_running';
    }

    if (section !== '1_history') handledSinceBoot.add(scheduleID);
    rows.push({
      ...schedule,
      row_key: `task-${task.id}`,
      task,
      section,
    });
  });

  bootSchedules.forEach((schedule) => {
    const scheduleID = Number(schedule.id);
    if (
      eligibleIDs.has(scheduleID)
      && !handledSinceBoot.has(scheduleID)
    ) {
      rows.push({
        ...schedule,
        row_key: `schedule-${scheduleID}-pending`,
        task: null,
        section: '4_pending',
      });
    }
  });

  return rows.sort((a, b) => {
    const sectionOrder = a.section.localeCompare(b.section);
    return sectionOrder || (Number(b.task && b.task.id) || 0) - (Number(a.task && a.task.id) || 0);
  });
}
