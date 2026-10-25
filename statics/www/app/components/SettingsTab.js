// Settings tab: defaults, storage, durability and deletion.

import { api, collectionUrl, findCollection, notify } from '../store.js';
import { formatBytes, formatNumber, prettyJson } from '../format.js';
import { COLLECTION_KEY } from '../query.js';
import { Icon } from '../icons.js';
import { DURABILITY } from './NewCollectionDialog.js';

const { computed, inject, reactive } = Vue;

const formatDefaults = (defaults) => (defaults && Object.keys(defaults).length ? prettyJson(defaults) : '{\n  \n}');

const metricLabel = (key) => {
  if (key === 'disk') return 'Journal file on disk';
  if (key === 'memory') return 'Documents in memory';
  if (key.startsWith('index.')) return `Index ${key.slice(6)}`;
  return key;
};

export const SettingsTab = {
  name: 'SettingsTab',
  components: { Icon },
  emits: ['drop'],
  setup() {
    const ctx = inject(COLLECTION_KEY);
    const initial = formatDefaults(findCollection(ctx.name)?.defaults);
    const defaults = reactive({ draft: initial, original: initial, saving: false, error: '' });

    const saveDefaults = async () => {
      defaults.error = '';
      let parsed = {};
      if (defaults.draft.trim()) {
        try {
          parsed = JSON.parse(defaults.draft);
        } catch (err) {
          defaults.error = `Invalid JSON: ${err.message}`;
          return;
        }
      }
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        defaults.error = 'Defaults must be a JSON object.';
        return;
      }
      defaults.saving = true;
      try {
        const { data } = await api('Update defaults', 'POST', collectionUrl(ctx.name, 'setDefaults'), { body: parsed });
        const saved = data && typeof data === 'object' && !Array.isArray(data) ? data : parsed;
        defaults.original = formatDefaults(saved);
        defaults.draft = defaults.original;
        const collection = findCollection(ctx.name);
        if (collection) collection.defaults = saved;
        notify('success', 'Defaults saved.');
      } catch (err) {
        defaults.error = err.message || 'The defaults could not be saved.';
      } finally {
        defaults.saving = false;
      }
    };

    const metrics = computed(() =>
      Object.entries(ctx.metrics.data || {})
        .filter(([, v]) => typeof v === 'number' && v >= 0)
        .map(([key, value]) => ({ key, label: metricLabel(key), value: formatBytes(value), title: `${formatNumber(value)} bytes` })),
    );
    const durability = computed(() => DURABILITY[ctx.collection.value?.durability === 'strong' ? 'strong' : 'relaxed']);

    return { ctx, defaults, saveDefaults, metrics, durability };
  },
  template: `
    <div class="stack-lg">
      <section class="card">
        <div class="card-head">
          <div>
            <h2 class="card-title">Default values</h2>
            <p class="card-sub">Fields added to inserted documents that do not include them.</p>
          </div>
        </div>
        <div class="card-body">
          <div class="fields" style="grid-template-columns:minmax(0,2fr) minmax(220px,1fr)">
            <div>
              <label for="defaults-json" class="sr-only">Defaults (JSON)</label>
              <textarea id="defaults-json" v-model="defaults.draft" class="textarea" rows="8" spellcheck="false"></textarea>
              <p v-if="defaults.error" class="msg-error">{{ defaults.error }}</p>
            </div>
            <div class="panel small">
              <span class="strong">Generated values</span>
              <div><code>"uuid()"</code> <span class="muted">a random unique id</span></div>
              <div><code>"auto()"</code> <span class="muted">an increasing number</span></div>
              <div><code>"unixnano()"</code> <span class="muted">the insert time in nanoseconds</span></div>
              <span class="muted">Any other value is copied as it is.</span>
            </div>
          </div>
          <div class="row end">
            <button type="button" class="btn" :disabled="defaults.saving || defaults.draft === defaults.original" @click="defaults.draft = defaults.original">Discard changes</button>
            <button type="button" class="btn btn-primary" :disabled="defaults.saving || defaults.draft === defaults.original" @click="saveDefaults">{{ defaults.saving ? 'Saving…' : 'Save defaults' }}</button>
          </div>
        </div>
      </section>

      <div class="fields" style="grid-template-columns:repeat(auto-fit,minmax(300px,1fr))">
        <section class="card">
          <div class="card-head">
            <h2 class="card-title">Storage</h2>
            <button type="button" class="btn btn-sm" :disabled="ctx.metrics.loading" @click="ctx.refreshMetrics">{{ ctx.metrics.loading ? 'Refreshing…' : 'Refresh' }}</button>
          </div>
          <div class="card-body">
            <p v-if="ctx.metrics.error" class="msg-error">{{ ctx.metrics.error }}</p>
            <dl v-else-if="metrics.length" class="kv">
              <div v-for="m in metrics" :key="m.key"><dt>{{ m.label }}</dt><dd :title="m.title">{{ m.value }}</dd></div>
            </dl>
            <p v-else class="small muted">No storage figures reported.</p>
          </div>
        </section>
        <section class="card">
          <div class="card-head"><h2 class="card-title">Durability</h2></div>
          <div class="card-body">
            <div class="row"><icon name="shield" /><span class="strong">{{ durability.title }}</span></div>
            <p class="small muted">{{ durability.desc }}</p>
            <p class="tiny faint">Durability is chosen when the collection is created. Single requests can still ask for a disk write with <code>?wait=true</code>.</p>
          </div>
        </section>
      </div>

      <section class="card" style="border-color:var(--danger)">
        <div class="card-head" style="border-bottom:0">
          <div>
            <h2 class="card-title">Delete collection</h2>
            <p class="card-sub">Removes the collection, its documents and indexes permanently.</p>
          </div>
          <button type="button" class="btn btn-danger" @click="$emit('drop')"><icon name="trash" /> Delete collection</button>
        </div>
      </section>
    </div>`,
};
