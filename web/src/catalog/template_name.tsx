import type { MouseEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { leaveIsGuarded, requestLeave } from '../shell/leave_guard';
import './template_name.css';

export interface TemplateNotice {
  readonly label: string;
  readonly path: string;
}

export interface TemplateNameProps {
  readonly name: string;
  readonly notice?: TemplateNotice;
}

function isPlainActivation(event: MouseEvent<HTMLAnchorElement>): boolean {
  return event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;
}

export function TemplateName({ name, notice }: TemplateNameProps) {
  const navigate = useNavigate();

  if (notice === undefined) {
    return <>{name}</>;
  }

  return (
    <>
      {name}
      <span className="lg-template-name__separator" aria-hidden="true">
        {' · '}
      </span>
      <Link
        className="lg-template-name__notice"
        to={notice.path}
        onClick={(event) => {
          if (!isPlainActivation(event) || !leaveIsGuarded()) {
            return;
          }
          event.preventDefault();
          requestLeave(() => navigate(notice.path));
        }}
      >
        {notice.label}
      </Link>
    </>
  );
}
