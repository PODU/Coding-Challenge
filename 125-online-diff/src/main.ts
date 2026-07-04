import './styles.css';
import { createApp } from './ui/app';

const mount = document.getElementById('app');
if (!mount) throw new Error('missing #app mount point');
createApp(mount);
