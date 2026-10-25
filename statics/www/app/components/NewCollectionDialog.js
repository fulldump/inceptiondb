// Dialog to create a collection.

import { api, collections, loadCollections, notify } from '../store.js';
import { AppDialog } from './common.js';

const { reactive, watch } = Vue;

export const DURABILITY = {
  relaxed: {
    title: 'Relaxed',
    desc: 'Fastest. Writes are acknowledged once in memory; the latest ones can be lost if the server crashes.',
  },
  strong: {
    title: 'Strong',
    desc: 'Writes are acknowledged only after they reach the disk. They survive crashes and power loss.',
  },
};

export const NewCollectionDialog = {
  name: 'NewCollectionDialog',
  components: { AppDialog },
  props: { open: { type: Boolean, default: false } },
  emits: ['update:open', 'created'],
  setup(props, { emit }) {
    const form = reactive({ name: '', durability: 'relaxed', error: '', submitting: false });

    watch(
      () => props.open,
      (open) => {
        if (open) Object.assign(form, { name: '', durability: 'relaxed', error: '', submitting: false });
      },
    );

    const submit = async () => {
      const name = form.name.trim();
      form.error = '';
      if (!name) {
        form.error = 'Give the collection a name.';
        return;
      }
      if (collections.list.some((c) => c.name === name)) {
        form.error = `A collection named "${name}" already exists.`;
        return;
      }
      form.submitting = true;
      try {
        await api(`Create collection ${name}`, 'POST', '/v1/collections', { body: { name, durability: form.durability } });
        await loadCollections();
        notify('success', `Collection "${name}" created.`);
        emit('update:open', false);
        emit('created', name);
      } catch (err) {
        form.error = err.message || 'The collection could not be created.';
      } finally {
        form.submitting = false;
      }
    };

    return { form, submit, durability: DURABILITY };
  },
  template: `
    <app-dialog :open="open" labelledby="new-collection-title" @update:open="$emit('update:open', $event)">
      <form @submit.prevent="submit">
        <div class="modal-head">
          <p id="new-collection-title" class="modal-title">New collection</p>
          <p class="small muted">Collections hold JSON documents. You can add indexes later.</p>
        </div>
        <div class="modal-body">
          <div>
            <label for="new-collection-name" class="label">Name</label>
            <input id="new-collection-name" v-model="form.name" type="text" class="input" placeholder="customers" autocomplete="off" autofocus>
          </div>
          <div>
            <span class="label">Durability</span>
            <div class="choice-grid" role="radiogroup" aria-label="Durability">
              <button v-for="(d, key) in durability" :key="key" type="button" class="choice" role="radio"
                :aria-checked="form.durability === key" :aria-pressed="form.durability === key" @click="form.durability = key">
                <span class="title">{{ d.title }}</span>
                <span class="desc">{{ d.desc }}</span>
              </button>
            </div>
          </div>
          <p v-if="form.error" class="msg-error">{{ form.error }}</p>
        </div>
        <div class="modal-foot">
          <button type="button" class="btn" @click="$emit('update:open', false)">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="form.submitting">{{ form.submitting ? 'Creating…' : 'Create collection' }}</button>
        </div>
      </form>
    </app-dialog>`,
};
