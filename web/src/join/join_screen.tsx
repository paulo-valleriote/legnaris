import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import type { JoinedCampaign } from '../api/models/campaign';
import { failureCopy, failureKind } from '../api/error_mapper';
import { useApiClient } from '../app/providers';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Field } from '../ui/field';
import { PageHeader } from '../ui/page_header';
import { ErrorState } from '../ui/state';
import './join.css';

export function JoinScreen() {
  const { code = '' } = useParams();
  const client = useApiClient();
  const [password, setPassword] = useState('');
  const [passwordRequired, setPasswordRequired] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const passwordRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (attempt > 0 && passwordRequired) {
      passwordRef.current?.focus();
    }
  }, [attempt, passwordRequired]);

  const join = useMutation<JoinedCampaign, unknown, void>({
    mutationFn: () =>
      client.joinCampaign({ code, password: passwordRequired ? password : undefined }),
    onMutate: () => {
      setFailure(null);
    },
    onError: (error) => {
      if (failureKind(error) === 'passwordRequired') {
        setPasswordRequired(true);
        setFailure(null);
      } else {
        setFailure(failureCopy(error));
      }
      setAttempt((value) => value + 1);
    },
  });

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (join.isPending || (passwordRequired && password === '')) {
      return;
    }
    join.mutate();
  };

  if (join.isSuccess) {
    return (
      <section className="lg-join">
        <PageHeader title={message('join.joinedTitle')} />
        <p className="lg-join__message">{messageWith('join.joined', { name: join.data.name })}</p>
        <Button
          variant="secondary"
          size="md"
          className="lg-join__action"
          label={message('join.backToSheets')}
          to="/"
        />
      </section>
    );
  }

  const passwordMissing = passwordRequired && password === '';
  const showFailure = failure !== null && !passwordRequired;

  return (
    <section className="lg-join">
      <PageHeader title={message('join.title')} />
      <p className="lg-join__message">{message('join.intro')}</p>
      <form className="lg-join__form" onSubmit={onSubmit}>
        {showFailure ? <ErrorState message={failure} /> : null}
        {passwordRequired ? (
          <Field
            label={message('join.passwordLabel')}
            id="join-password"
            name="password"
            type="password"
            autoComplete="off"
            value={password}
            hint={message('join.passwordHint')}
            error={failure ?? undefined}
            inputRef={passwordRef}
            onChange={setPassword}
          />
        ) : null}
        <Button
          type="submit"
          fullWidth
          loading={join.isPending}
          label={join.isPending ? message('join.submitting') : message('join.submit')}
          disabledReason={passwordMissing ? message('join.submitNeedsPassword') : undefined}
        />
      </form>
    </section>
  );
}
