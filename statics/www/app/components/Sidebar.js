// Sidebar: collection navigation, connection status and request log.

import { activity, checkConnection, collections, connection, showRequest } from '../store.js';
import { formatTime, prettyTotal } from '../format.js';
import { Icon } from '../icons.js';

const { computed, ref } = Vue;

const STATUS_LABEL = { online: 'Connected', degraded: 'Degraded', offline: 'Offline', unknown: 'Checking…' };

export const Sidebar = {
  name: 'Sidebar',
  components: { Icon },
  props: { selected: { type: String, default: '' }, open: { type: Boolean, default: false } },
  emits: ['select', 'create', 'home'],
  setup() {
    const search = ref('');
    const filtered = computed(() => {
      const term = search.value.trim().toLowerCase();
      return term ? collections.list.filter((c) => c.name.toLowerCase().includes(term)) : collections.list;
    });
    const statusLabel = computed(() => STATUS_LABEL[connection.state] || 'Unknown');
    const statusTitle = computed(() =>
      [connection.detail, connection.checkedAt ? `checked at ${formatTime(connection.checkedAt)}` : ''].filter(Boolean).join(' · '),
    );
    const failedRequests = computed(() => activity.entries.filter((e) => e.status === 'error').length);
    const openActivity = () => showRequest();
    return { collections, connection, activity, search, filtered, statusLabel, statusTitle, failedRequests, checkConnection, openActivity, prettyTotal };
  },
  template: `
    <aside class="sidebar" :class="{ 'is-open': open }" aria-label="Navigation">
      <div class="sidebar-top">
        <a href="#/" class="brand" @click="$emit('home')"><span class="brand-mark">IDB</span>InceptionDB</a>
        <div class="search">
          <icon name="search" :size="14" />
          <label for="collection-search" class="sr-only">Search collections</label>
          <input id="collection-search" v-model="search" type="search" class="input input-sm" placeholder="Search collections" autocomplete="off">
        </div>
      </div>

      <div class="sidebar-section">
        <span>Collections</span>
        <button type="button" class="btn btn-ghost btn-sm btn-icon" aria-label="New collection" title="New collection" @click="$emit('create')">
          <icon name="plus" :size="15" />
        </button>
      </div>
      <nav class="sidebar-list" aria-label="Collections">
        <p v-if="collections.loading && !collections.list.length" class="tiny faint" style="padding:8px 10px">Loading…</p>
        <p v-else-if="collections.error" class="msg-error" style="padding:8px 10px">{{ collections.error }}</p>
        <template v-else>
          <button v-for="c in filtered" :key="c.name" type="button" class="nav-item"
            :class="{ 'is-active': c.name === selected }" :aria-current="c.name === selected ? 'page' : null"
            :title="c.name" @click="$emit('select', c.name)">
            <icon name="table" :size="15" class="nav-icon" />
            <span class="grow ellipsis">{{ c.name }}</span>
            <icon v-if="c.durability === 'strong'" name="shield" :size="13" class="nav-icon" title="Strong durability" />
            <span class="count">{{ prettyTotal(c.total) }}</span>
          </button>
          <p v-if="!collections.list.length" class="tiny faint" style="padding:8px 10px">No collections yet.</p>
          <p v-else-if="!filtered.length" class="tiny faint" style="padding:8px 10px">No collection matches "{{ search }}".</p>
        </template>
      </nav>

      <div class="sidebar-footer">
        <button type="button" class="footer-button" :title="statusTitle" @click="checkConnection">
          <span class="dot" :class="['status-' + connection.state, { 'is-pulsing': connection.checking }]"></span>
          <span class="grow">{{ statusLabel }}</span>
          <span class="tiny faint">{{ connection.checking ? 'checking' : 'check' }}</span>
        </button>
        <button type="button" class="footer-button" @click="openActivity">
          <icon name="braces" :size="15" />
          <span class="grow">API calls</span>
          <span v-if="failedRequests" class="badge badge-danger">{{ failedRequests }} failed</span>
          <span v-else class="tiny faint">{{ activity.entries.length }}</span>
        </button>
        <a class="footer-button" href="https://github.com/fulldump/inceptiondb" target="_blank" rel="noopener">
          <icon name="github" :size="15" /><span class="grow">GitHub</span>
        </a>
      </div>
    </aside>`,
};
