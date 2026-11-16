import { Outlet } from 'react-router-dom';
import { useStageThemeColor } from '../landing/use_stage_theme_color';
import { LeftNav } from './left_nav';
import './shell.css';

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
