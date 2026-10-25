// Home: server overview, collections and a quick start.

import { collections, connection, notify } from '../store.js';
import { formatNumber, formatTime } from '../format.js';
import { Icon } from '../icons.js';

const { computed } = Vue;

export const Home = {
  name: 'Home',
  components: { Icon },
  emits: ['select', 'create'],
  setup() {
    const totalDocuments = computed(() => collections.list.reduce((sum, c) => sum + (c.total || 0), 0));
    const strongCount = computed(() => collections.list.filter((c) => c.durability === 'strong').length);
    const origin = window.location.origin;
    const quickStart = [
      '# Insert documents (the collection is created if it does not exist)',
      `curl -X POST '${origin}/v1/collections/people:insert' \\`,
      `  -d '{"id":"ada","name":"Ada Lovelace","born":1815}'`,
      '',
      '# Query them',
      `curl -X POST '${origin}/v1/collections/people:find' \\`,
      `  -d '{"filter":{"name":"Ada Lovelace"}}'`,
    ].join('\n');
    const statusLabel = computed(() => ({ online: 'Online', degraded: 'Degraded', offline: 'Offline' })[connection.state] || 'Checking…');

    const copy = async () => {
      try {
        await navigator.clipboard.writeText(quickStart);
        notify('success', 'Commands copied to the clipboard.');
      } catch (err) {
        notify('error', 'The clipboard is not available in this browser.');
      }
    };

    return { collections, connection, totalDocuments, strongCount, quickStart, statusLabel, copy, formatNumber, formatTime };
  },
  template: `
    <div class="content">
      <div class="page-head">
        <div>
          <h1 class="page-title">Overview</h1>
          <p class="page-sub">Collections and health of this InceptionDB server.</p>
        </div>
        <button type="button" class="btn btn-primary" @click="$emit('create')"><icon name="plus" /> New collection</button>
      </div>

      <div class="stats">
        <div class="stat">
          <div class="stat-label"><span class="dot" :class="'status-' + connection.state"></span> Server</div>
          <div class="stat-value">{{ statusLabel }}</div>
          <div class="stat-note">{{ connection.checkedAt ? 'Checked at ' + formatTime(connection.checkedAt) : connection.detail }}</div>
        </div>
        <div class="stat">
          <div class="stat-label"><icon name="layers" :size="13" /> Collections</div>
          <div class="stat-value">{{ formatNumber(collections.list.length) }}</div>
          <div class="stat-note">{{ strongCount }} with strong durability</div>
        </div>
        <div class="stat">
          <div class="stat-label"><icon name="database" :size="13" /> Documents</div>
          <div class="stat-value">{{ formatNumber(totalDocuments) }}</div>
          <div class="stat-note">Across all collections</div>
        </div>
      </div>

      <section class="stack">
        <h2 class="card-title">Collections</h2>
        <div v-if="!collections.list.length && !collections.loading" class="card">
          <div class="empty">
            <div class="empty-icon"><icon name="database" :size="22" /></div>
            <h3>No collections yet</h3>
            <p>A collection stores JSON documents. Create one here, or insert documents through the API and it will be created for you.</p>
            <button type="button" class="btn btn-primary" @click="$emit('create')"><icon name="plus" /> Create your first collection</button>
          </div>
        </div>
        <div v-else class="collection-grid">
          <button v-for="c in collections.list" :key="c.name" type="button" class="collection-card" @click="$emit('select', c.name)">
            <div class="row between">
              <span class="name ellipsis">{{ c.name }}</span>
              <span v-if="c.durability === 'strong'" class="badge badge-accent"><icon name="shield" :size="11" /> Strong</span>
            </div>
            <div>
              <div class="figure">{{ formatNumber(c.total || 0) }}</div>
              <div class="tiny muted">documents · {{ c.indexes || 0 }} {{ c.indexes === 1 ? 'index' : 'indexes' }}</div>
            </div>
          </button>
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <div>
            <h2 class="card-title">Quick start</h2>
            <p class="card-sub">Use the HTTP API from any language. Every action in this console shows its request in the request log.</p>
          </div>
          <button type="button" class="btn btn-sm" @click="copy"><icon name="copy" /> Copy</button>
        </div>
        <div class="card-body">
          <pre class="code-block">{{ quickStart }}</pre>
        </div>
      </section>
    </div>`,
};
