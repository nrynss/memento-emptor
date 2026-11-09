import { mount } from 'svelte';
import 'ag-grid-community/styles/ag-grid.css';
import '@nrynss/chaaya/tokens/reference.css';
import './demo.css';
import DemoApp from './DemoApp.svelte';

mount(DemoApp, { target: document.getElementById('app') ?? document.body });
