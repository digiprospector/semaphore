import { expect } from 'chai';
import { getBootScheduleEntries, getTodayScheduleEntries } from '@/lib/scheduleOverview';
import ScheduleOverview from '@/views/project/ScheduleOverview.vue';

describe('lib/scheduleOverview', () => {
  it('reloads overview data when switching between today and boot modes', () => {
    expect(ScheduleOverview.watch.mode).to.equal('loadSchedules');
  });

  it('includes active cron schedules with occurrences today in the schedule timezone', () => {
    const schedules = [
      {
        id: 1,
        active: true,
        type: '',
        cron_format: '0 9 * * *',
      },
      {
        id: 2,
        active: true,
        type: '',
        cron_format: '0 9 * * 1',
      },
      {
        id: 3,
        active: false,
        type: '',
        cron_format: '* * * * *',
      },
    ];

    const result = getTodayScheduleEntries(schedules, 'Asia/Shanghai', new Date('2026-11-03T00:00:00Z'));

    expect(result.map((schedule) => schedule.id)).to.deep.equal([1]);
    expect(result[0].first_run_at).to.equal('2026-11-03T01:00:00.000Z');
  });

  it('excludes weekly cron schedules when their next occurrence is not today', () => {
    const result = getTodayScheduleEntries([
      {
        id: 1,
        active: true,
        type: '',
        cron_format: '0 9 * * 1',
      },
    ], 'Asia/Shanghai', new Date('2026-11-03T00:00:00Z'));

    expect(result).to.have.length(0);
  });

  it('includes run_at schedules on the selected local date', () => {
    const result = getTodayScheduleEntries([
      {
        id: 1,
        active: true,
        type: 'run_at',
        run_at: '2026-11-03T01:00:00.000Z',
      },
    ], 'Asia/Shanghai', new Date('2026-11-03T00:00:00Z'));

    expect(result).to.have.length(1);
  });

  it('keeps a completed one-time schedule visible today and attaches its task', () => {
    const result = getTodayScheduleEntries([
      {
        id: 1,
        active: false,
        type: 'run_at',
        run_at: '2026-11-03T01:00:00.000Z',
      },
    ], 'Asia/Shanghai', new Date('2026-11-03T00:00:00Z'), [
      {
        id: 12,
        schedule_id: 1,
        created: '2026-11-03T01:00:00.000Z',
        status: 'success',
      },
    ]);

    expect(result).to.have.length(1);
    expect(result[0].task.id).to.equal(12);
  });

  it('does not attach a task from a previous local day', () => {
    const result = getTodayScheduleEntries([
      {
        id: 1,
        active: true,
        type: '',
        cron_format: '0 9 * * *',
      },
    ], 'Asia/Shanghai', new Date('2026-11-03T00:00:00Z'), [
      {
        id: 12,
        schedule_id: 1,
        created: '2026-11-02T01:00:00.000Z',
        start: '2026-11-02T01:00:01.000Z',
        end: '2026-11-02T01:00:02.000Z',
        status: 'success',
      },
    ]);

    expect(result).to.have.length(1);
    expect(result[0].task).to.equal(null);
  });

  it('shows every task and every unrun cron occurrence for today', () => {
    const result = getTodayScheduleEntries([
      {
        id: 1,
        active: true,
        type: '',
        cron_format: '0 9,12,15 * * *',
      },
    ], 'Asia/Shanghai', new Date('2026-11-03T00:00:00Z'), [
      {
        id: 21,
        schedule_id: 1,
        created: '2026-11-03T01:00:00.000Z',
        start: '2026-11-03T01:00:03.000Z',
        status: 'success',
      },
      {
        id: 22,
        schedule_id: 1,
        created: '2026-11-03T04:00:00.000Z',
        start: '2026-11-03T04:00:03.000Z',
        status: 'running',
      },
    ]);

    expect(result.map((entry) => entry.task && entry.task.id)).to.deep.equal([21, 22, null]);
    expect(result.map((entry) => entry.first_run_at)).to.deep.equal([
      '2026-11-03T01:00:00.000Z',
      '2026-11-03T04:00:00.000Z',
      '2026-11-03T07:00:00.000Z',
    ]);
    expect(new Set(result.map((entry) => entry.row_key)).size).to.equal(3);
  });

  it('hides unrun schedule occurrences earlier than now', () => {
    const result = getTodayScheduleEntries([
      {
        id: 1,
        active: true,
        type: '',
        cron_format: '0 9,12,15 * * *',
      },
    ], 'Asia/Shanghai', new Date('2026-11-03T04:30:00Z'));

    expect(result.map((entry) => entry.first_run_at)).to.deep.equal([
      '2026-11-03T07:00:00.000Z',
    ]);
    expect(result[0].task).to.equal(null);
  });

  it('groups every on_boot task instance around the current process boot', () => {
    const result = getBootScheduleEntries([
      {
        id: 1,
        template_id: 7,
        active: true,
        type: 'on_boot',
        name: 'first',
      },
      {
        id: 2,
        template_id: 7,
        active: true,
        type: 'on_boot',
        name: 'second',
      },
      {
        id: 5,
        template_id: 7,
        active: true,
        type: 'on_boot',
        name: 'third',
      },
      {
        id: 3,
        template_id: 8,
        active: false,
        type: 'on_boot',
        name: 'disabled',
      },
      {
        id: 4,
        template_id: 9,
        active: true,
        type: '',
        name: 'cron',
      },
    ], [
      {
        id: 11,
        schedule_id: 1,
        created: '2026-01-01T00:00:00.000Z',
        start: '2026-01-01T00:00:01.000Z',
        end: '2026-01-01T00:00:02.000Z',
        status: 'success',
      },
      {
        id: 12,
        schedule_id: 1,
        created: '2026-03-01T00:00:00.000Z',
        start: '2026-03-01T00:00:01.000Z',
        end: '2026-03-01T00:00:02.000Z',
        status: 'success',
      },
      {
        id: 13,
        schedule_id: 1,
        created: '2026-03-02T00:00:00.000Z',
        start: '2026-03-02T00:00:01.000Z',
        status: 'running',
      },
      {
        id: 14,
        schedule_id: 1,
        created: '2026-01-02T00:00:00.000Z',
        start: '2026-01-02T00:00:01.000Z',
        end: '2026-01-02T00:00:02.000Z',
        status: 'success',
      },
      {
        id: 15,
        schedule_id: 2,
        created: '2026-03-03T00:00:00.000Z',
        status: 'waiting',
      },
    ], '2026-02-01T00:00:00.000Z', [1, 2, 5]);

    expect(result.map((entry) => entry.section)).to.deep.equal([
      '1_history',
      '1_history',
      '2_completed',
      '3_running',
      '4_pending',
      '4_pending',
    ]);
    expect(result.map((entry) => entry.task && entry.task.id)).to.deep.equal([
      14,
      11,
      12,
      13,
      15,
      null,
    ]);
    expect(result.map((entry) => entry.template_id)).to.deep.equal([7, 7, 7, 7, 7, 7]);
    expect(new Set(result.map((entry) => entry.row_key)).size).to.equal(6);
  });

  it('formats a pending boot task start as a countdown and scheduled local time', () => {
    const bootTime = new Date(Date.now() + 30 * 60 * 1000 + 30 * 1000).toISOString();
    const result = ScheduleOverview.methods.formatExpectedStart.call({
      bootTime,
      timezoneName: 'Asia/Shanghai',
      $t: (key, params) => `${params.count} minutes later`,
    }, { time_after_boot: 60 });

    expect(result).to.match(/^32 minutes later \(\d{2}:\d{2}\)$/);
  });
});
