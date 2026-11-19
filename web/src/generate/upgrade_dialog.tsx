import {
  PAID_GENERATIONS_PER_DAY,
  PAID_OWNED_CAMPAIGNS,
  PAID_SHEET_LIMITS,
} from '../api/models/plan';
import { PLANS_PATH } from '../app/routes';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import './upgrade_dialog.css';

export function UpgradeDialog({
  open,
  onClose,
}: {
  readonly open: boolean;
  readonly onClose: () => void;
}) {
  return (
    <Dialog
      open={open}
      title={message('generate.upgrade.title')}
      description={message('generate.upgrade.body')}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" label={message('common.notNow')} onClick={onClose} />
          <Button variant="primary" label={message('generate.upgrade.seePlans')} to={PLANS_PATH} />
        </>
      }
    >
      <dl className="lg-upgrade__limits">
        <div className="lg-upgrade__limit lg-upgrade__limit--highlight">
          <dt>{message('generate.upgrade.aiTitle')}</dt>
          <dd>{messageWith('generate.upgrade.aiValue', { limit: PAID_GENERATIONS_PER_DAY })}</dd>
        </div>
        <div className="lg-upgrade__limit">
          <dt>{message('generate.upgrade.sheetsTitle')}</dt>
          <dd>
            {messageWith('generate.upgrade.sheetsValue', {
              characters: PAID_SHEET_LIMITS.character,
              npcs: PAID_SHEET_LIMITS.npc,
              enemies: PAID_SHEET_LIMITS.enemy,
            })}
          </dd>
        </div>
        <div className="lg-upgrade__limit">
          <dt>{message('generate.upgrade.campaignsTitle')}</dt>
          <dd>{messageWith('generate.upgrade.campaignsValue', { limit: PAID_OWNED_CAMPAIGNS })}</dd>
        </div>
      </dl>
    </Dialog>
  );
}
