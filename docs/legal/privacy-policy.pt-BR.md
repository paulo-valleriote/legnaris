<!--
  Política de privacidade da Legnaris (geral). Geração com IA: LGN-228, ADR 0008. Pagamentos: LGN-222, ADR 0009.
  Local: docs/legal/privacy-policy.pt-BR.md (português). Inglês: docs/legal/privacy-policy.en.md.
  As cópias web em web/src/legal/ devem permanecer idênticas byte a byte a estes arquivos.
  O texto de consentimento de IA é docs/legal/llm-consent.json (versão "3"); a seção 6 deve permanecer consistente com ele.
  Suba a versão do consentimento apenas quando os operadores de IA, o modelo, a região, a retenção ou o caminho de revogação mudarem.
  Status: FINAL (versão 4, 16/11/2026).
-->

# Política de privacidade

Última atualização: 16/11/2026. Versão da política: 4.

Esta política descreve como a Legnaris trata seus dados pessoais quando você cria uma conta, mantém Fichas de personagem, assina o Plano pago e usa o recurso opcional de geração de personagem com IA.

## 1. Controlador

O controlador dos seus dados pessoais é Paulo Valleriote, pessoa física, que pode ser contatado em paulovalleriotedev@gmail.com.

## 2. Dados que tratamos e por quê

- **Conta:** seu endereço de e-mail, nome de usuário e senha. A senha é armazenada apenas como hash. Usamos esses dados para criar sua conta, fazer seu login e verificar seu endereço de e-mail. Base legal: execução do contrato com você (LGPD art. 7º, V).
- **Fichas:** os personagens e o conteúdo que você cria. Nós os armazenamos para prestar o serviço. Base legal: execução do contrato (LGPD art. 7º, V).
- **Login e segurança:** tokens de sessão, códigos de verificação por e-mail de curta duração e logs técnicos de requisições, que incluem seu endereço IP, o user agent do navegador e o endereço acessado. Usamos esses dados para manter sua conta segura, evitar abusos e corrigir erros. Base legal: legítimo interesse (LGPD art. 7º, IX).
- **Plano pago:** o status da sua assinatura, os identificadores de cliente e de assinatura na Stripe, o período de cobrança, o status de reembolso e as notificações de pagamento que a Stripe nos envia. Usamos esses dados para liberar o Plano pago, tratar cancelamentos e reembolsos e manter registros contábeis. Base legal: execução do contrato e cumprimento de obrigação legal (LGPD art. 7º, V e II).
- **Geração de personagem com IA (opcional):** o texto do conceito que você digita e o resultado gerado (seção 6). Base legal: seu consentimento (LGPD art. 7º, I).

## 3. Pagamentos

Os pagamentos do Plano pago são processados pela **Stripe** (Stripe, Inc. e suas afiliadas). Você informa os dados do cartão em uma página hospedada pela Stripe. A Legnaris nunca recebe nem armazena o número do seu cartão. A Stripe coleta os dados de que precisa para processar o pagamento, como cartão, nome, endereço de e-mail e país de cobrança, e os trata conforme a própria política de privacidade (https://stripe.com/privacy).

A Stripe envia à Legnaris notificações sobre a sua assinatura. Mantemos as notificações brutas por 90 dias e depois as apagamos automaticamente. Mantemos o registro da assinatura enquanto sua conta existir e, para fins fiscais e contábeis, por 5 anos após você excluí-la.

## 4. Operadores

Compartilhamos dados pessoais apenas com os operadores de que precisamos para prestar o serviço:

- **Stripe**, processamento de pagamentos do Plano pago (seção 3).
- **Resend**, serviço de envio de e-mails nos Estados Unidos, que envia códigos de verificação e e-mails da conta para o seu endereço de e-mail.
- **OpenRouter** e os **provedores do modelo** que ela seleciona, apenas se você usar a geração de personagem com IA e der seu consentimento (seção 6).

Não vendemos seus dados pessoais e não usamos cookies de publicidade ou de análise.

## 5. Onde seus dados ficam e transferências internacionais

A Legnaris armazena sua conta, suas Fichas, os registros de assinatura e os logs em servidores no Brasil.

Alguns operadores tratam dados fora do Brasil, o que é transferência internacional de dados pessoais nos termos do art. 33 da Lei Geral de Proteção de Dados (LGPD):

- **Stripe e Resend** podem tratar dados nos Estados Unidos e em outros países. A transferência é necessária para a execução do contrato com você (LGPD art. 33, IX).
- **A geração de personagem com IA** pode ser processada em outros países, inclusive na China (seção 6). A transferência se baseia no seu consentimento específico e destacado (LGPD art. 33, VIII).

## 6. Geração de personagem com IA

Quando você pede à Legnaris para gerar um personagem, o texto do conceito que você digita é enviado a um modelo de IA. O resultado gerado é devolvido a você. A Legnaris armazena ambos por no máximo 24 horas e nunca os grava em logs.

Para executar o recurso, usamos:

- **OpenRouter**, um gateway de modelos que recebe sua requisição e a encaminha. Pessoa jurídica: OpenRouter, Inc., Estados Unidos.
- **Provedores do modelo**, as empresas que executam o modelo DeepSeek V4.1 Flash, selecionadas pela OpenRouter para cada requisição. O provedor pode ser a API da DeepSeek ou outra empresa, localizada em países diferentes, inclusive na China. A OpenRouter publica a lista de provedores e suas localizações.

As requisições usam o roteamento padrão da OpenRouter, com alternativas entre provedores. O roteamento não é restrito por retenção de dados; portanto, um provedor que atenda a sua requisição pode reter prompts e resultados, ou usá-los em treinamento, conforme os termos próprios dele. A Legnaris não controla esses termos. Você pode consultá-los na documentação da OpenRouter e no site de cada provedor.

Só iniciamos uma geração depois que você dá seu consentimento específico e destacado no app. Você pode retirar seu consentimento a qualquer momento, na tela Conta (/account) ou pela API em DELETE /me/llm-consent. A retirada:

- apaga os dados de geração que a Legnaris ainda mantém sobre você, e
- interrompe qualquer nova transferência do seu conceito à OpenRouter e aos provedores do modelo.

A retirada não apaga o conteúdo já presente na sua Ficha. Você pode apagar esse conteúdo na própria Ficha. A retirada não afeta o restante da sua conta. Você pode dar o consentimento novamente a qualquer momento.

## 7. Retenção

- **Conta e Fichas:** até você apagá-las ou excluir sua conta.
- **Tokens de sessão:** até 30 dias.
- **Códigos de verificação por e-mail:** apagados automaticamente depois que expiram.
- **Registros dos e-mails enviados a você:** 7 dias.
- **Logs técnicos de requisições:** até 31 dias.
- **Notificações de pagamento da Stripe:** 90 dias.
- **Registros de assinatura:** enquanto sua conta existir e por 5 anos após você excluí-la, para cumprir obrigações fiscais e contábeis.
- **Dados de geração com IA:** até 24 horas. O conteúdo gerado que você mantém na sua própria Ficha permanece lá até você apagá-lo.
- **Nos operadores:** a Stripe, a Resend, a OpenRouter e os provedores do modelo mantêm dados conforme os termos próprios deles. A Stripe pode manter registros de pagamento pelo prazo que a lei exigir.

## 8. Exclusão da conta

Você pode excluir sua conta pela API em DELETE /me ou escrevendo para paulovalleriotedev@gmail.com. A exclusão remove da Legnaris sua conta, suas Fichas, seus dados de geração e as notificações de pagamento. Cancelamos qualquer assinatura ativa na Stripe e removemos seu nome, endereço de e-mail e telefone do seu cadastro de cliente na Stripe. Mantemos o registro da assinatura, que não contém e-mail nem nome, por 5 anos para cumprir obrigações fiscais e contábeis. A Stripe pode manter os registros de pagamento que a lei a obriga a manter.

## 9. Cookies

A Legnaris usa apenas os cookies necessários para manter você conectado e proteger sua sessão contra falsificação de requisições entre sites. Ela não usa cookies de publicidade ou de análise.

## 10. Seus direitos

Pela LGPD, você pode solicitar a confirmação do tratamento, acesso, correção, anonimização, portabilidade, eliminação, informação sobre as entidades com as quais compartilhamos seus dados e informação sobre a possibilidade de não fornecer consentimento e sobre as consequências dessa negativa. Envie suas solicitações para paulovalleriotedev@gmail.com.

## 11. Contato do encarregado de dados

Fundador da Legnaris (encarregado de proteção de dados), paulovalleriotedev@gmail.com.

## 12. Alterações

Quando esta política mudar, nós a atualizaremos junto com a versão no início deste documento. Quando os operadores de IA, o modelo, a região de processamento ou o período de retenção da geração de personagem com IA mudarem, também atualizaremos o texto de consentimento e pediremos que você consinta novamente.
