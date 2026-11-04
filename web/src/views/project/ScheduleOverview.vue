<template>
  <div>
    <v-toolbar flat>
      <v-app-bar-nav-icon @click="showDrawer()"></v-app-bar-nav-icon>
      <v-toolbar-title>{{ mode === 'today' ? $t('today') : $t('boot') }}</v-toolbar-title>
      <v-spacer />
      <v-btn icon :to="`/project/${projectId}/schedule`" :title="$t('manageSchedules')">
        <v-icon>mdi-calendar-edit</v-icon>
      </v-btn>
    </v-toolbar>

    <v-alert v-if="loadError" type="error" dense class="ma-4">
      {{ loadError }}
      <v-btn icon small class="ml-2" :title="$t('audit_retry')" @click="loadSchedules()">
        <v-icon small>mdi-refresh</v-icon>
      </v-btn>
    </v-alert>

    <v-data-table
      :headers="headers"
      :items="items"
      item-key="row_key"
      :group-by="mode === 'boot' ? ['section'] : []"
      :loading="loading"
      :items-per-page="20"
      :footer-props="{ itemsPerPageOptions: [20, 50, 100] }"
      class="mt-4"
    >
      <template v-slot:group.header="{ group, items, isOpen, toggle }">
        <td :colspan="headers.length">
          <v-btn icon small @click="toggle">
            <v-icon>{{ isOpen ? 'mdi-chevron-down' : 'mdi-chevron-right' }}</v-icon>
          </v-btn>
          {{ sectionTitle(group) }} ({{ items.length }})
        </td>
      </template>
      <template v-slot:item.name="{ item }">
        {{ item.name || '—' }}
      </template>
      <template v-slot:item.template="{ item }">
        <router-link :to="`/project/${projectId}/templates/${item.template_id}`">
          {{ item.tpl_name || `#${item.template_id}` }}
        </router-link>
      </template>
      <template v-slot:item.timing="{ item }">
        <span v-if="mode === 'boot'">{{ formatDelay(item.time_after_boot) }}</span>
        <span v-else>{{ formatRunTime(item.first_run_at) }}</span>
      </template>
      <template v-slot:item.task="{ item }">
        <TaskLink
          v-if="item.task"
          :task-id="item.task.id"
          :label="'#' + item.task.id"
        />
        <span v-else>{{ $t('notRunYet') }}</span>
      </template>
      <template v-slot:item.status="{ item }">
        <TaskStatus v-if="item.task" :status="item.task.status" />
        <v-chip v-else style="font-weight: bold;">{{ $t('scheduledStatus') }}</v-chip>
      </template>
      <template v-slot:item.start="{ item }">
        <span v-if="item.task && item.task.start">{{ item.task.start | formatDate }}</span>
        <span v-else-if="mode === 'boot'">{{ formatExpectedStart(item) }}</span>
        <span v-else>&mdash;</span>
      </template>
      <template v-slot:item.duration="{ item }">
        <span v-if="item.task && item.task.start">
          {{ [item.task.start, item.task.end] | formatMilliseconds }}
        </span>
        <span v-else>&mdash;</span>
      </template>
      <template v-slot:no-data>
        {{ mode === 'today' ? $t('noSchedulesToday') : $t('noBootSchedules') }}
      </template>
    </v-data-table>
  </div>
</template>

<script>
import axios from 'axios';
import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc';
import timezone from 'dayjs/plugin/timezone';
import EventBus from '@/event-bus';
import socket from '@/socket';
import TaskLink from '@/components/TaskLink.vue';
import TaskStatus from '@/components/TaskStatus.vue';
import { getBootScheduleEntries, getTodayScheduleEntries } from '@/lib/scheduleOverview';
import { resolveTimezone } from '@/lib/timezone';

dayjs.extend(utc);
dayjs.extend(timezone);

export default {
  components: { TaskLink, TaskStatus },

  props: {
    projectId: Number,
    mode: {
      type: String,
      required: true,
      validator: (value) => ['today', 'boot'].includes(value),
    },
    systemInfo: Object,
  },

  data() {
    return {
      schedules: [],
      tasks: [],
      bootTime: null,
      eligibleScheduleIDs: [],
      loading: false,
      loadError: null,
      reloadTimer: null,
    };
  },

  computed: {
    timezoneName() {
      return resolveTimezone((this.systemInfo || {}).schedule_timezone);
    },
    items() {
      if (this.mode === 'today') {
        return getTodayScheduleEntries(this.schedules, this.timezoneName, new Date(), this.tasks);
      }
      return getBootScheduleEntries(
        this.schedules,
        this.tasks,
        this.bootTime,
        this.eligibleScheduleIDs,
      );
    },
    headers() {
      const headers = [
        { text: this.$t('name'), value: 'name' },
        { text: this.$t('template'), value: 'template' },
        { text: this.$t('scheduledFor'), value: 'timing' },
        { text: this.$t('task2'), value: 'task' },
        { text: this.$t('status'), value: 'status' },
        { text: this.$t('start'), value: 'start' },
        { text: this.$t('duration'), value: 'duration' },
      ];
      return headers;
    },
  },

  watch: {
    projectId: 'loadSchedules',
    mode: 'loadSchedules',
  },

  created() {
    this.socketListenerId = socket.addListener((data) => this.onWebsocketDataReceived(data));
    this.loadSchedules();
  },

  beforeDestroy() {
    socket.removeListener(this.socketListenerId);
    if (this.reloadTimer) clearTimeout(this.reloadTimer);
  },

  methods: {
    showDrawer() {
      EventBus.$emit('i-show-drawer');
    },
    async loadSchedules() {
      if (!this.projectId) return;
      this.loading = true;
      this.loadError = null;
      try {
        if (this.mode === 'boot') {
          const response = await axios({
            method: 'get',
            url: `/api/project/${this.projectId}/schedules/on_boot/overview`,
            responseType: 'json',
          });
          this.schedules = response.data.schedules || [];
          this.tasks = response.data.tasks || [];
          this.bootTime = response.data.boot_time;
          this.eligibleScheduleIDs = response.data.eligible_schedule_ids || [];
        } else {
          const response = await axios({
            method: 'get',
            url: `/api/project/${this.projectId}/schedules/today/overview`,
            responseType: 'json',
          });
          this.schedules = response.data.schedules || [];
          this.tasks = response.data.tasks || [];
        }
      } catch (error) {
        this.schedules = [];
        this.tasks = [];
        this.bootTime = null;
        this.eligibleScheduleIDs = [];
        this.loadError = this.$t('schedulesLoadFailed');
      } finally {
        this.loading = false;
      }
    },
    async onWebsocketDataReceived(data) {
      if (data.project_id !== this.projectId || data.type !== 'update') return;

      const task = this.tasks.find((item) => item.id === data.task_id);
      if (task) {
        Object.assign(task, { ...data, type: undefined });
        return;
      }

      if (!this.reloadTimer) {
        this.reloadTimer = setTimeout(() => {
          this.reloadTimer = null;
          this.loadSchedules();
        }, 5000);
      }
    },
    formatDelay(seconds) {
      const value = Math.max(0, Number(seconds) || 0);
      if (value < 60) return this.$t('secondsAfterStart', { count: value });
      const minutes = Math.floor(value / 60);
      const remainingSeconds = value % 60;
      if (remainingSeconds === 0) return this.$t('minutesAfterStart', { count: minutes });
      return this.$t('minutesSecondsAfterStart', { minutes, seconds: remainingSeconds });
    },
    formatRunTime(value) {
      return dayjs(value).tz(this.timezoneName).format('HH:mm');
    },
    formatExpectedStart(item) {
      if (!this.bootTime) return '\u2014';
      const scheduledStart = dayjs(this.bootTime).add(Number(item.time_after_boot) || 0, 'second');
      const minutes = Math.max(0, Math.ceil(scheduledStart.diff(dayjs()) / 60000));
      return `${this.$t('minutesUntilStart', { count: minutes })} (${scheduledStart.tz(this.timezoneName).format('HH:mm')})`;
    },
    sectionTitle(section) {
      const keys = {
        '1_history': 'bootHistoricalTasks',
        '2_completed': 'bootCompletedTasks',
        '3_running': 'bootRunningTasks',
        '4_pending': 'bootPendingTasks',
      };
      return this.$t(keys[section]);
    },
  },
};
</script>
