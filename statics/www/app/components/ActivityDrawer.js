// Request log: the latest API calls made by the console and how to reproduce them.

import { activity, notify, ui } from '../store.js';
import { SNIPPETS } from '../snippets.js';
import { formatDuration } from '../format.js';
import { Icon } from '../icons.js';
import { AppDialog } from './common.js';

const { computed, ref, watch } = Vue;

// The language the user picked is reused for every request, also after a reload.
const LANGUAGE_KEY = 'inceptiondb.snippetLanguage';
const DEFAULT_LANGUAGE = 'curl';

const loadLanguage = () => {
  try {
    const saved = window.localStorage.getItem(LANGUAGE_KEY);
    return SNIPPETS.some((s) => s.id === saved) ? saved : DEFAULT_LANGUAGE;
  } catch (err) {
    return DEFAULT_LANGUAGE; // storage blocked (private mode, policies)
  }
};

const saveLanguage = (id) => {
  try {
    window.localStorage.setItem(LANGUAGE_KEY, id);
  } catch (err) {
    // Not persisted; the choice still applies during this visit.
  }
};

export const ActivityDrawer = {
  name: 'ActivityDrawer',
  components: { AppDialog, Icon },
  setup() {
    const selected = ref(null);
    const tab = ref(loadLanguage());

    // The log opens on the request chosen from the rail or a notification.
    watch(
      () => [ui.activityOpen, ui.activityEntry],
      ([open, entry]) => {
        selected.value = open ? entry : null;
      },
    );
    watch(selected, (entry) => {
      if (ui.activityOpen) ui.activityEntry = entry;
    });

    const tabs = computed(() =>
      selected.value ? SNIPPETS.map((s) => ({ id: s.id, label: s.label, content: s.build(selected.value.request) })).filter((t) => t.content) : [],
    );
    const selectLanguage = (id) => {
      tab.value = id;
      saveLanguage(id);
    };

    const content = computed(() => (tabs.value.find((t) => t.id === tab.value) || tabs.value[0] || {}).content || '');

    const statusBadge = (entry) =>
      entry.status === 'success' ? 'badge badge-success' : entry.status === 'error' ? 'badge badge-danger' : 'badge badge-accent';
    const statusText = (entry) => (entry.status === 'pending' ? '…' : entry.statusCode || 'ERR');

    const copy = async () => {
      try {
        await navigator.clipboard.writeText(content.value);
        notify('info', 'Snippet copied to the clipboard.');
      } catch (err) {
        notify('info', 'The clipboard is not available here; select the snippet and copy it.');
      }
    };

    return { ui, activity, selected, tab, tabs, content, selectLanguage, statusBadge, statusText, copy, formatDuration };
  },
  template: `
    <app-dialog :open="ui.activityOpen" variant="drawer" labelledby="activity-title" @update:open="ui.activityOpen = $event">
      <div class="drawer-head">
        <div class="stack-sm">
          <h2 id="activity-title" class="drawer-title">{{ selected ? selected.label : 'API calls' }}</h2>
          <p class="small muted">{{ selected ? 'This is the exact request the console sent. Copy it to do the same from your code.' : 'Everything the console does is a plain HTTP request. Pick one to see its code.' }}</p>
        </div>
        <div class="row">
          <button v-if="selected" type="button" class="btn btn-sm" @click="selected = null"><icon name="chevronLeft" /> All requests</button>
          <button type="button" class="btn btn-ghost btn-sm btn-icon" aria-label="Close" @click="ui.activityOpen = false"><icon name="x" /></button>
        </div>
      </div>
      <div class="drawer-body">
        <template v-if="!selected">
          <p v-if="!activity.entries.length" class="small muted">No requests yet.</p>
          <div v-else class="request-list">
            <button v-for="entry in activity.entries" :key="entry.id" type="button" class="request" @click="selected = entry">
              <span class="method">{{ entry.request.method }}</span>
              <span class="stack-sm" style="gap:0">
                <span class="ellipsis strong small">{{ entry.label }}</span>
                <span class="ellipsis tiny faint mono">{{ entry.request.url }}</span>
              </span>
              <span class="row">
                <span class="tiny faint" v-if="entry.durationMs !== null">{{ formatDuration(entry.durationMs) }}</span>
                <span :class="statusBadge(entry)">{{ statusText(entry) }}</span>
              </span>
            </button>
          </div>
        </template>
        <template v-else>
          <div class="row wrap small">
            <span :class="statusBadge(selected)">{{ statusText(selected) }}</span>
            <span class="mono strong">{{ selected.request.method }}</span>
            <span class="mono muted break grow">{{ selected.request.url }}</span>
            <span class="faint" v-if="selected.durationMs !== null">{{ formatDuration(selected.durationMs) }}</span>
          </div>
          <div class="tab-panel">
            <div class="tabs" role="tablist">
              <button v-for="t in tabs" :key="t.id" type="button" role="tab" :aria-selected="tab === t.id" @click="selectLanguage(t.id)">{{ t.label }}</button>
            </div>
            <pre role="tabpanel">{{ content }}</pre>
          </div>
          <div class="row end"><button type="button" class="btn btn-sm" @click="copy"><icon name="copy" /> Copy snippet</button></div>
        </template>
      </div>
    </app-dialog>`,
};
