// Root component: layout, hash routing and connection polling.

import { checkConnection, collections, findCollection, loadCollections } from '../store.js';
import { buildRoute, parseRoute } from '../route.js';
import { Icon } from '../icons.js';
import { Sidebar } from './Sidebar.js';
import { Home } from './Home.js';
import { CollectionPage } from './CollectionPage.js';
import { NewCollectionDialog } from './NewCollectionDialog.js';
import { ActivityDrawer } from './ActivityDrawer.js';
import { ActivityRail } from './ActivityRail.js';
import { ConfirmDialog, Toasts } from './common.js';

const { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } = Vue;

const STATUS_POLL_MS = 30000;

const sameQuery = (a, b) => a.skip === b.skip && a.limit === b.limit && a.query === b.query;

export const App = {
  name: 'App',
  components: { Icon, Sidebar, Home, CollectionPage, NewCollectionDialog, ActivityDrawer, ActivityRail, ConfirmDialog, Toasts },
  setup() {
    const route = shallowRef(parseRoute(window.location.hash));
    const routeVersion = ref(0);
    const sidebarOpen = ref(false);
    const creating = ref(false);
    const collectionsLoaded = ref(false);
    let writtenHash = null;

    const selected = computed(() => route.value.collection);
    const selectedExists = computed(() => !!findCollection(selected.value));

    const onHashChange = () => {
      // The hashchange caused by navigate() is already applied. Consume the
      // marker so a later visit to the same hash (back/forward) is applied.
      if (writtenHash !== null && window.location.hash === writtenHash) {
        writtenHash = null;
        return;
      }
      writtenHash = null;
      const next = parseRoute(window.location.hash);
      const previous = route.value;
      route.value = next;
      // Same collection with another query (back/forward): re-run it.
      // A tab change alone keeps the current results.
      if (next.collection === previous.collection && !sameQuery(next, previous)) routeVersion.value += 1;
    };

    // navigate records the state of the open collection in the URL.
    const navigate = (state) => {
      const hash = buildRoute(state);
      if (window.location.hash === hash) return;
      writtenHash = hash;
      window.location.hash = hash;
      route.value = parseRoute(hash);
    };

    const select = (name) => {
      sidebarOpen.value = false;
      window.location.hash = buildRoute({ collection: name });
    };
    const goHome = () => {
      sidebarOpen.value = false;
      window.location.hash = '#/';
    };

    watch(
      selected,
      () => {
        document.title = selected.value ? `${selected.value} · InceptionDB` : 'InceptionDB Console';
      },
      { immediate: true },
    );

    let poller = null;
    onMounted(async () => {
      window.addEventListener('hashchange', onHashChange);
      poller = window.setInterval(checkConnection, STATUS_POLL_MS);
      await loadCollections();
      collectionsLoaded.value = true;
    });
    onBeforeUnmount(() => {
      window.removeEventListener('hashchange', onHashChange);
      window.clearInterval(poller);
    });

    return { route, routeVersion, selected, selectedExists, collections, collectionsLoaded, sidebarOpen, creating, navigate, select, goHome };
  },
  template: `
    <div class="layout">
      <sidebar :selected="selected" :open="sidebarOpen" @select="select" @create="creating = true" @home="sidebarOpen = false" />
      <div v-if="sidebarOpen" class="sidebar-backdrop" @click="sidebarOpen = false"></div>

      <main class="main">
        <div class="topbar">
          <button type="button" class="btn btn-ghost btn-icon" aria-label="Open navigation" @click="sidebarOpen = true"><icon name="menu" :size="20" /></button>
          <span class="strong ellipsis">{{ selected || 'InceptionDB' }}</span>
        </div>

        <collection-page v-if="selected && selectedExists" :key="selected" :name="selected"
          :route="route" :route-version="routeVersion" @navigate="navigate" @dropped="goHome" />

        <div v-else-if="selected && collectionsLoaded && !collections.loading" class="content">
          <div class="card">
            <div class="empty">
              <div class="empty-icon"><icon name="search" :size="22" /></div>
              <h3>Collection not found</h3>
              <p>There is no collection named "{{ selected }}" on this server.</p>
              <button type="button" class="btn" @click="goHome">Go to overview</button>
            </div>
          </div>
        </div>

        <home v-else-if="!selected" @select="select" @create="creating = true" />
      </main>

      <new-collection-dialog v-model:open="creating" @created="select" />
      <activity-rail />
      <activity-drawer />
      <confirm-dialog />
      <toasts />
    </div>`,
};
