// Always visible trail of API calls. Every action in the console is an HTTP
// request; the rail makes that visible so users can repeat it from code.

import { activity, showRequest, ui } from '../store.js';
import { Icon } from '../icons.js';

const { computed, ref, watch } = Vue;

export const ActivityRail = {
  name: 'ActivityRail',
  components: { Icon },
  setup() {
    const newestId = computed(() => activity.entries[0]?.id ?? null);
    // Bumped on every new request to restart the attention animation.
    const bump = ref(0);
    watch(newestId, (id, previous) => {
      if (id !== null && id !== previous) bump.value += 1;
    });

    const title = (entry) => {
      const status = entry.status === 'pending' ? 'in progress' : entry.statusCode ? `HTTP ${entry.statusCode}` : 'failed';
      return `${entry.label} · ${entry.request.method} ${entry.request.url} · ${status} — click to see the code`;
    };

    return { activity, ui, newestId, bump, title, showRequest };
  },
  template: `
    <div class="activity-rail" aria-label="API calls made by this console">
      <button type="button" class="rail-head" title="API calls: see how to make each one from your code" @click="showRequest()">
        <icon name="braces" :size="14" />
      </button>
      <div class="rail-markers" role="list">
        <button v-for="entry in activity.entries" :key="entry.id" type="button" role="listitem"
          class="activity-marker"
          :class="['is-' + entry.status, { 'is-background': entry.background, 'is-newest': entry.id === newestId, 'is-selected': ui.activityOpen && ui.activityEntry && ui.activityEntry.id === entry.id }]"
          :data-label="entry.label" :aria-label="title(entry)" :title="title(entry)" @click="showRequest(entry)"></button>
      </div>
    </div>
    <button v-if="activity.entries.length" :key="'fab-' + bump" type="button" class="activity-fab"
      :class="'is-' + (activity.entries[0] ? activity.entries[0].status : 'success')"
      aria-label="API calls made by this console" @click="showRequest()">
      <icon name="braces" :size="18" />
      <span class="fab-count">{{ activity.entries.length }}</span>
    </button>`,
};
