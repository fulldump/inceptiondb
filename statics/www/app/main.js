// Entry point. Vue is loaded as a global by index.html (lib/vue.global.prod.js,
// the full build that compiles the component templates in the browser).

import { App } from './components/App.js';
import { checkConnection } from './store.js';

Vue.createApp(App).mount('#app');
checkConnection();
