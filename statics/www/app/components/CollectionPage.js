// Page of one collection: header, tabs and side panels. Mounted with
// :key="name", so switching collections starts from a clean state.

import { api, collectionUrl, confirmDanger, findCollection, loadCollections, notify } from '../store.js';
import { COLLECTION_KEY, createQuery, QUERY_KEY } from '../query.js';
import { buildRoute } from '../route.js';
import { formatNumber, prettyTotal } from '../format.js';
import { Icon } from '../icons.js';
import { DropdownMenu } from './common.js';
import { DocumentsTab } from './DocumentsTab.js';
import { IndexesTab } from './IndexesTab.js';
import { SettingsTab } from './SettingsTab.js';
import { DocumentDrawer } from './DocumentDrawer.js';
import { InsertDrawer, ImportDrawer } from './WriteDrawers.js';

const { computed, onMounted, provide, reactive, watch } = Vue;

export const CollectionPage = {
  name: 'CollectionPage',
  components: { Icon, DropdownMenu, DocumentsTab, IndexesTab, SettingsTab, DocumentDrawer, InsertDrawer, ImportDrawer },
  props: {
    name: { type: String, required: true },
    route: { type: Object, required: true },
    routeVersion: { type: Number, default: 0 },
  },
  emits: ['navigate', 'dropped'],
  setup(props, { emit }) {
    const query = createQuery(props.name, {
      route: props.route,
      onNavigate: (next) => emit('navigate', { ...next, tab: props.route.tab }),
    });
    provide(QUERY_KEY, query);

    const collection = computed(() => findCollection(props.name));
    const metrics = reactive({ data: null, loading: false, error: '' });
    const panels = reactive({ document: null, insert: false, import: null }); // import: csv | jsonl

    const refreshMetrics = async () => {
      metrics.loading = true;
      metrics.error = '';
      try {
        const { data } = await api('Get collection size', 'POST', collectionUrl(props.name, 'size'), { background: true });
        metrics.data = data && typeof data === 'object' ? data : {};
      } catch (err) {
        metrics.error = err.message;
      } finally {
        metrics.loading = false;
      }
    };

    // refresh reloads everything that a write can change.
    const refresh = async () => {
      await Promise.all([query.run(), loadCollections(), refreshMetrics()]);
    };

    const ctx = {
      name: props.name,
      collection,
      metrics,
      refreshMetrics,
      refresh,
      openDocument: (doc) => {
        panels.document = doc;
      },
      openInsert: () => {
        panels.insert = true;
      },
      openImport: (format = 'csv') => {
        panels.import = format;
      },
    };
    provide(COLLECTION_KEY, ctx);

    // Back/forward navigation within the same collection.
    watch(
      () => props.routeVersion,
      () => query.applyRoute(props.route),
    );

    onMounted(() => {
      query.loadIndexes();
      query.run();
      refreshMetrics();
    });

    const tabHref = (tab) => buildRoute({ collection: props.name, tab, skip: query.state.appliedSkip, limit: query.state.appliedLimit, query: query.state.filterText });

    const drop = async () => {
      const count = collection.value?.total ?? 0;
      if (!(await confirmDanger(`Delete "${props.name}"?`, `The collection and its ${formatNumber(count)} documents will be permanently deleted. This cannot be undone.`, 'Delete collection'))) return;
      try {
        await api(`Delete collection ${props.name}`, 'POST', collectionUrl(props.name, 'dropCollection'));
        notify('success', `Collection "${props.name}" deleted.`);
        emit('dropped');
        await loadCollections();
      } catch (err) {
        notify('error', err.message || 'The collection could not be deleted.');
      }
    };

    return { query, state: query.state, collection, metrics, panels, ctx, tabHref, drop, formatNumber, prettyTotal };
  },
  template: `
    <div class="content">
      <div class="page-head">
        <div>
          <h1 class="page-title">
            <span class="break">{{ name }}</span>
            <a v-if="collection" :href="tabHref('settings')" class="badge" :class="{ 'badge-accent': collection.durability === 'strong' }"
              :title="collection.durability === 'strong'
                ? 'Writes are acknowledged after they reach the disk. Click for details.'
                : 'Writes are acknowledged once in memory; the latest can be lost on a crash. Click for details.'">
              <icon name="shield" :size="12" /> {{ collection.durability === 'strong' ? 'Strong durability' : 'Relaxed durability' }}
            </a>
          </h1>
        </div>
        <div class="row wrap">
          <button type="button" class="btn btn-primary" @click="ctx.openInsert"><icon name="plus" /> Insert documents</button>
          <dropdown-menu label="Collection actions">
            <button type="button" role="menuitem" @click="ctx.openImport('csv')">Import CSV…</button>
            <button type="button" role="menuitem" @click="ctx.openImport('jsonl')">Import JSONL…</button>
            <hr class="divider">
            <button type="button" role="menuitem" @click="ctx.refresh">Refresh</button>
            <button type="button" role="menuitem" class="is-danger" @click="drop">Delete collection</button>
          </dropdown-menu>
        </div>
      </div>

      <nav class="tabs-nav" aria-label="Collection sections">
        <a :href="tabHref('documents')" :aria-current="route.tab === 'documents' ? 'page' : null"><icon name="table" :size="15" /> Documents
          <span v-if="collection" class="badge" :title="formatNumber(collection.total) + ' documents'">{{ prettyTotal(collection.total) }}</span></a>
        <a :href="tabHref('indexes')" :aria-current="route.tab === 'indexes' ? 'page' : null"><icon name="zap" :size="15" /> Indexes <span class="badge">{{ state.indexes.length }}</span></a>
        <a :href="tabHref('settings')" :aria-current="route.tab === 'settings' ? 'page' : null"><icon name="settings" :size="15" /> Settings</a>
      </nav>

      <documents-tab v-show="route.tab === 'documents'" />
      <indexes-tab v-if="route.tab === 'indexes'" />
      <settings-tab v-if="route.tab === 'settings'" @drop="drop" />

      <document-drawer :document="panels.document" @update:document="panels.document = $event" />
      <insert-drawer v-model:open="panels.insert" />
      <import-drawer v-model:format="panels.import" />
    </div>`,
};
