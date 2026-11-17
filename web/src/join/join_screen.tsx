import { useState, type FormEvent } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import type { JoinedCampaign } from '../api/models/campaign';
import { failureCopy, failureKind } from '../api/error_mapper';
import { useApiClient } from '../app/providers';
import { message, messageWith } from '../l10n/strings';
import './join.css';

export function JoinScreen() {
  const { code = '' } = useParams();
  const client = useApiClient();
  const [password, setPassword] = useState('');
  const [passwordRequired, setPasswordRequired] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  const join = useMutation<JoinedCampaign, unknown, void>({
    mutationFn: () =>
      client.joinCampaign({ code, password: passwordRequired ? password : undefined }),
    onMutate: () => {
      setFailure(null);
    },
    onError: (error) => {
      if (failureKind(error) === 'passwordRequired') {
        setPasswordRequired(true);
      }
      setFailure(failureCopy(error));
    },
  });

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (join.isPending) {
      return;
    }
    join.mutate();
  };

  if (join.isSuccess) {
    return (
      <section className="lg-join" aria-labelledby="join-title">
        <h1 className="lg-join__title" id="join-title">
          {message('join.joinedTitle')}
        </h1>
        <p className="lg-join__message">{messageWith('join.joined', { name: join.data.name })}</p>
        <Link to="/">{message('join.backToSheets')}</Link>
      </section>
    );
  }

  const submitDisabled = join.isPending || (passwordRequired && password === '');

  return (
    <section className="lg-join" aria-labelledby="join-title">
      <h1 className="lg-join__title" id="join-title">
        {message('join.title')}
      </h1>
      <p className="lg-join__message">{message('join.intro')}</p>
      <form className="lg-join__form" onSubmit={onSubmit}>
        {failure === null ? null : (
          <p className="lg-join__failure" role="alert">
            {failure}
          </p>
        )}
        {passwordRequired ? (
          <label className="lg-join__field">
            <span>{message('join.passwordLabel')}</span>
            <input
              className="lg-join__input"
              type="password"
              name="password"
              autoComplete="off"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
        ) : null}
        <button type="submit" className="lg-button" disabled={submitDisabled}>
          {message(join.isPending ? 'join.submitting' : 'join.submit')}
        </button>
      </form>
    </section>
  );
}
