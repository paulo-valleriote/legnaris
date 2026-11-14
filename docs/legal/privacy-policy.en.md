<!--
  Privacy policy update: LLM character generation (LGN-228, ADR 0008).
  Location: docs/legal/privacy-policy.en.md (English). Portuguese: docs/legal/privacy-policy.pt-BR.md.
  The consent disclosure shown in the app is docs/legal/llm-consent.json (version "1"), which must stay consistent with this text.
  Status: DRAFT, pending legal review. Every [SQUARE_BRACKETS] token is a placeholder that legal must fill in; no legal entity name or address is invented here.
-->

# Privacy policy: AI character generation

Last updated: [DATE]. Policy version: 1.

This section supplements the Legnaris Privacy Policy [MAIN_PRIVACY_POLICY_URL]. It describes how the optional AI character generation feature handles your personal data.

## 1. Controller

The controller of your personal data is [CONTROLLER_LEGAL_NAME], [CONTROLLER_ADDRESS], [CONTROLLER_CNPJ_OR_EQUIVALENT].

## 2. What the feature processes

When you ask Legnaris to generate a character, the concept text you type is sent to an AI model. The generated output is returned to you. Legnaris stores both only for a short period (section 5) and never writes them to logs.

## 3. Processors

To run the feature we use two processors:

- **OpenRouter**, a model gateway that receives your request and routes it. Legal entity: [OPENROUTER_LEGAL_NAME], [OPENROUTER_COUNTRY].
- **[SERVING_PROVIDER]**, the provider that runs the DeepSeek V4.1 Flash model that OpenRouter selects for your request. Legal entity: [SERVING_PROVIDER_LEGAL_NAME], [SERVING_PROVIDER_COUNTRY].

Requests are configured so that OpenRouter only routes to providers with zero data retention and no training on your data, and never to the first-party DeepSeek API. Legal review must confirm these terms with the processors' current data processing agreements.

## 4. International transfer

Processing happens outside Brazil, in [PROCESSING_COUNTRIES]. This is an international transfer of personal data under article 33 of the Brazilian General Data Protection Law (LGPD). We rely on your specific and separate consent, which you give in the app before your first generation, as the legal basis for this transfer. We do not start a transfer without that consent.

## 5. Retention

- **In Legnaris:** the concept and the generated output are kept for up to 24 hours and then deleted automatically.
- **At the processors:** the processors are configured not to retain prompts or outputs and not to use them for training. [RETENTION_AT_PROCESSORS_CONFIRMED_BY_LEGAL]
- **On your Sheet:** generated content that you keep on your own Sheet stays there until you delete it.

Your data export does not include the generation data, because it is deleted within 24 hours.

## 6. Withdrawing consent

You can withdraw your consent to AI processing at any time, in [REVOKE_SCREEN_PATH] or through [REVOKE_API_PATH]. Withdrawing:

- deletes the generation data that Legnaris still holds for you, and
- stops any further transfer of your concept to the processors.

Withdrawing does not delete content already on your Sheet. You can delete that content from the Sheet. Withdrawing does not affect the rest of your account. You can give consent again at any time.

## 7. Your rights

Under the LGPD you can request confirmation of processing, access, correction, anonymization, portability, deletion, information about the entities we share your data with, and information about the right to refuse consent and its consequences. Send requests to [PRIVACY_CONTACT_EMAIL].

## 8. Data protection contact

[DATA_PROTECTION_OFFICER_NAME_OR_ROLE], [DATA_PROTECTION_CONTACT_EMAIL].

## 9. Changes

When the processors, the model, the processing region or the retention period changes, we update this policy and the consent disclosure, and we ask you to consent again. Changes are identified by the policy version at the top of this document.
