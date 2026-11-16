import { Link } from 'react-router-dom';
import { PLANS_PATH } from '../app/routes';
import { useCachedPlanUsage } from '../catalog/queries';
import { message } from '../l10n/strings';
import './payment_banner.css';

export function PaymentIssueBanner() {
  const plan = useCachedPlanUsage();

  if (plan.data?.billing.paymentIssue !== true) {
    return null;
  }

  return (
    <div className="lg-banner" role="status">
      <p className="lg-banner__text">{message('banner.paymentIssue')}</p>
      <Link className="lg-banner__link" to={PLANS_PATH}>
        {message('banner.paymentAction')}
      </Link>
    </div>
  );
}
