<!--
  Legnaris privacy policy (general). AI generation: LGN-228, ADR 0008. Payments: LGN-222, ADR 0009.
  Location: docs/legal/privacy-policy.en.md (English). Portuguese: docs/legal/privacy-policy.pt-BR.md.
  The web copies in web/src/legal/ must stay byte-identical to these files.
  The AI consent disclosure is docs/legal/llm-consent.json (version "3"); section 6 must stay consistent with it.
  Bump the consent version only when the AI processors, model, region, retention or revocation path change.
  Status: FINAL (version 4, 2026-11-16).
-->

# Privacy policy

Last updated: 2026-11-16. Policy version: 4.

This policy describes how Legnaris handles your personal data when you create an account, keep character Sheets, subscribe to the paid Plan and use the optional AI character generation.

## 1. Controller

The controller of your personal data is Paulo Valleriote, a natural person, who can be contacted at paulovalleriotedev@gmail.com.

## 2. Data we process and why

- **Account:** your email address, username and password. The password is stored only as a hash. We use this data to create your account, sign you in and verify your email address. Legal basis: performance of the contract with you (LGPD art. 7, V).
- **Sheets:** the characters and content you create. We store them to provide the service. Legal basis: performance of the contract (LGPD art. 7, V).
- **Sign-in and security:** session tokens, short-lived email verification codes and technical request logs, which include your IP address, browser user agent and the requested address. We use them to keep your account secure, prevent abuse and fix errors. Legal basis: legitimate interest (LGPD art. 7, IX).
- **Paid Plan:** your subscription status, the Stripe customer and subscription identifiers, billing period, refund status and the payment notifications Stripe sends us. We use them to grant the paid Plan, handle cancellations and refunds and keep accounting records. Legal basis: performance of the contract and compliance with legal obligations (LGPD art. 7, V and II).
- **AI character generation (optional):** the concept text you type and the generated output (section 6). Legal basis: your consent (LGPD art. 7, I).

## 3. Payments

Payments for the paid Plan are processed by **Stripe** (Stripe, Inc. and its affiliates). You enter your card details on a page hosted by Stripe. Legnaris never receives or stores your card number. Stripe collects the data it needs to process the payment, such as your card, name, email address and billing country, and handles it under its own privacy policy (https://stripe.com/privacy).

Stripe sends Legnaris notifications about your subscription. We keep the raw notifications for 90 days and then delete them automatically. We keep the subscription record while your account exists and, for tax and accounting purposes, for 5 years after you delete it.

## 4. Processors

We share personal data only with the processors we need to run the service:

- **Stripe**, payment processing for the paid Plan (section 3).
- **Resend**, an email delivery service in the United States, which sends verification codes and account emails to your email address.
- **OpenRouter** and the **model providers** it selects, only if you use AI character generation and give your consent (section 6).

We do not sell your personal data and we do not use advertising or analytics cookies.

## 5. Where your data is stored and international transfers

Legnaris stores your account, Sheets, subscription records and logs on servers in Brazil.

Some processors handle data outside Brazil, which is an international transfer of personal data under article 33 of the Brazilian General Data Protection Law (LGPD):

- **Stripe and Resend** may process data in the United States and other countries. The transfer is necessary to perform the contract with you (LGPD art. 33, IX).
- **AI character generation** may be processed in other countries, including China (section 6). The transfer relies on your specific and separate consent (LGPD art. 33, VIII).

## 6. AI character generation

When you ask Legnaris to generate a character, the concept text you type is sent to an AI model. The generated output is returned to you. Legnaris stores both only for up to 24 hours and never writes them to logs.

To run the feature we use:

- **OpenRouter**, a model gateway that receives your request and routes it. Legal entity: OpenRouter, Inc., United States.
- **Model providers**, the companies that run the DeepSeek V4.1 Flash model that OpenRouter selects for your request. The provider is chosen per request and may be the DeepSeek API or another company, located in different countries, including China. OpenRouter publishes the list of providers and their locations.

Requests use OpenRouter's default routing, with fallbacks between providers. Routing is not restricted by data retention, so a provider that serves your request may keep prompts and outputs, or use them for training, under its own terms. Legnaris does not control those terms. You can read them in OpenRouter's documentation and on each provider's website.

We start a generation only after you give your specific and separate consent in the app. You can withdraw your consent at any time, in the Account screen (/account) or through DELETE /me/llm-consent. Withdrawing:

- deletes the generation data that Legnaris still holds for you, and
- stops any further transfer of your concept to OpenRouter and the model providers.

Withdrawing does not delete content already on your Sheet. You can delete that content from the Sheet. Withdrawing does not affect the rest of your account. You can give consent again at any time.

## 7. Retention

- **Account and Sheets:** until you delete them or delete your account.
- **Session tokens:** up to 30 days.
- **Email verification codes:** deleted automatically after they expire.
- **Records of emails sent to you:** 7 days.
- **Technical request logs:** up to 31 days.
- **Payment notifications from Stripe:** 90 days.
- **Subscription records:** while your account exists and for 5 years after you delete it, to meet tax and accounting obligations.
- **AI generation data:** up to 24 hours. Generated content that you keep on your own Sheet stays there until you delete it.
- **At the processors:** Stripe, Resend, OpenRouter and the model providers keep data under their own terms. Stripe may keep payment records for as long as the law requires.

## 8. Deleting your account

You can delete your account through DELETE /me or by writing to paulovalleriotedev@gmail.com. Deletion removes your account, your Sheets, your generation data and the payment notifications from Legnaris. We cancel any active subscription at Stripe and remove your name, email address and phone number from your Stripe customer record. We keep the subscription record, which holds no email address or name, for 5 years to meet tax and accounting obligations. Stripe may keep payment records that the law requires it to keep.

## 9. Cookies

Legnaris uses only the cookies it needs to keep you signed in and to protect your session against cross-site request forgery. It does not use advertising or analytics cookies.

## 10. Your rights

Under the LGPD you can request confirmation of processing, access, correction, anonymization, portability, deletion, information about the entities we share your data with, and information about the right to refuse consent and its consequences. Send requests to paulovalleriotedev@gmail.com.

## 11. Data protection contact

Founder of Legnaris (data protection contact), paulovalleriotedev@gmail.com.

## 12. Changes

When this policy changes, we update it and its version at the top of this document. When the AI processors, the model, the processing region or the retention period of AI character generation changes, we also update the consent disclosure and ask you to consent again.
