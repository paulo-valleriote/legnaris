import { Outlet } from 'react-router-dom';
import { useStageThemeColor } from '../landing/use_stage_theme_color';
import { LeftNav } from './left_nav';
import { PaymentIssueBanner } from './payment_banner';
import './shell.css';

export function AppShell() {
  useStageThemeColor();

  return (
    <div className="lg-shell">
      <LeftNav />
      <main className="lg-content">
        <div className="lg-content__inner">
          <PaymentIssueBanner />
          <Outlet />
        </div>
      </main>
    </div>
  );
}
