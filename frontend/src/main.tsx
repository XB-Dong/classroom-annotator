import { createRoot } from 'react-dom/client';
import { App } from './app/App';
import './app/styles.css';
import './shared/ui/theme.css';
createRoot(document.getElementById('root')!).render(<App />);
