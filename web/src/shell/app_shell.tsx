import { Outlet } from 'react-router-dom';
import { LeftNav } from './left_nav';
import { RightPanel } from './right_panel';
import { useMediaQuery } from './use_media_query';
import { SHELL_NARROW_QUERY } from '../theme/tokens';
import './shell.css';

/**
 * The application shell: navigation, content, and contextual panel in order
 * (FND-01), with the panel collapsing first as the viewport narrows (FND-03).
 */
export function AppShell() {
  const narrow = useMediaQuery(SHELL_NARROW_QUERY);
  return (
    <div className={narrow ? 'lg-shell lg-shell--narrow' : 'lg-shell'}>
      <LeftNav />
      <main className="lg-content">
        <div className="lg-content__inner">
          <Outlet />
        </div>
      </main>
      <RightPanel collapsed={narrow} />
    </div>
  );
}
