import { Outlet } from 'react-router-dom';
import { useStageThemeColor } from '../landing/use_stage_theme_color';
import { LeftNav } from './left_nav';
import './shell.css';

/**
 * The application shell: navigation then content, exactly two regions on every
 * route (UIF-24). The navigation stays visible as the viewport narrows and the
 * content keeps its minimum width (UIF-25); there is no contextual panel.
 */
export function AppShell() {
  useStageThemeColor();

  return (
    <div className="lg-shell">
      <LeftNav />
      <main className="lg-content">
        <div className="lg-content__inner">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
