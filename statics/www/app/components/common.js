// Reusable components: dialogs, JSON view, confirmation, toasts and menus.

import { confirmation, dismiss, showRequest, toasts } from '../store.js';
import { highlightJson } from '../format.js';
import { Icon } from '../icons.js';

const { computed, onBeforeUnmount, onMounted, ref, watch } = Vue;

/**
 * AppDialog wraps a native <dialog> (focus trap, Esc to close). variant is
 * "modal" (centered) or "drawer" (side panel). Use v-model:open.
 */
export const AppDialog = {
  name: 'AppDialog',
  props: {
    open: { type: Boolean, default: false },
    variant: { type: String, default: 'modal' },
    wide: { type: Boolean, default: false },
    labelledby: { type: String, default: null },
  },
  emits: ['update:open'],
  setup(props, { emit }) {
    const el = ref(null);
    const sync = () => {
      if (!el.value) return;
      if (props.open && !el.value.open) el.value.showModal();
      if (!props.open && el.value.open) el.value.close();
    };
    watch(() => props.open, sync);
    onMounted(sync);
    const onClose = () => emit('update:open', false);
    // A click on the backdrop lands on the dialog element itself.
    const onClick = (event) => {
      if (event.target === el.value) emit('update:open', false);
    };
    const classes = computed(() => [props.variant === 'drawer' ? 'drawer' : 'modal', { 'modal-wide': props.wide }]);
    return { el, onClose, onClick, classes };
  },
  template: `
    <dialog ref="el" :class="classes" :aria-labelledby="labelledby" @close="onClose" @click="onClick">
      <slot v-if="open" />
    </dialog>`,
};

// Pretty printed, highlighted JSON. highlightJson escapes the content.
export const JsonView = {
  name: 'JsonView',
  props: { value: { required: true } },
  setup(props) {
    const html = computed(() => highlightJson(props.value));
    return { html };
  },
  template: `<pre class="json" v-html="html"></pre>`,
};

// Modal confirmation driven by confirmAction() in store.js.
export const ConfirmDialog = {
  name: 'ConfirmDialog',
  components: { AppDialog },
  setup() {
    const close = (value) => {
      const resolve = confirmation.resolve;
      confirmation.open = false;
      confirmation.resolve = null;
      if (resolve) resolve(value);
    };
    const onOpenChange = (open) => {
      if (!open && confirmation.open) close(null);
    };
    const buttonClass = (kind) => (kind === 'danger' ? 'btn btn-danger' : kind === 'primary' ? 'btn btn-primary' : 'btn');
    return { confirmation, close, onOpenChange, buttonClass };
  },
  template: `
    <app-dialog :open="confirmation.open" labelledby="confirm-title" @update:open="onOpenChange">
      <div class="modal-head">
        <p id="confirm-title" class="modal-title">{{ confirmation.title }}</p>
      </div>
      <div class="modal-body">
        <p class="muted">{{ confirmation.message }}</p>
      </div>
      <div class="modal-foot">
        <button type="button" class="btn" @click="close(null)">Cancel</button>
        <button v-for="choice in confirmation.choices" :key="String(choice.value)" type="button"
          :class="buttonClass(choice.kind)" @click="close(choice.value)">{{ choice.label }}</button>
      </div>
    </app-dialog>`,
};

export const Toasts = {
  name: 'Toasts',
  components: { Icon },
  setup() {
    const viewCode = (toast) => {
      dismiss(toast.id);
      showRequest(toast.entry);
    };
    return { toasts, dismiss, viewCode };
  },
  template: `
    <div class="toasts" role="status" aria-live="polite">
      <div v-for="t in toasts.list" :key="t.id" class="toast" :class="'toast-' + t.kind">
        <span class="dot"></span>
        <span class="grow break">{{ t.message }}</span>
        <button v-if="t.entry" type="button" class="btn btn-sm toast-code" title="See how to make this request from your code" @click="viewCode(t)">
          <icon name="braces" :size="13" /> View code
        </button>
        <button type="button" class="btn btn-ghost btn-sm btn-icon" aria-label="Dismiss" @click="dismiss(t.id)">×</button>
      </div>
    </div>`,
};

/** DropdownMenu shows its default slot (buttons) when the trigger is clicked. */
export const DropdownMenu = {
  name: 'DropdownMenu',
  components: { Icon },
  props: { label: { type: String, default: 'More actions' } },
  setup() {
    const open = ref(false);
    const root = ref(null);
    const onDocumentClick = (event) => {
      if (root.value && !root.value.contains(event.target)) open.value = false;
    };
    onMounted(() => document.addEventListener('click', onDocumentClick));
    onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick));
    return { open, root };
  },
  template: `
    <div ref="root" class="menu">
      <button type="button" class="btn btn-icon" :aria-label="label" :aria-expanded="open" @click="open = !open">
        <icon name="more" />
      </button>
      <div v-if="open" class="menu-list" role="menu" @click="open = false">
        <slot />
      </div>
    </div>`,
};
