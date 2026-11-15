<!--
  Atualização da política de privacidade: geração de personagem com LLM (LGN-228, ADR 0008).
  Local: docs/legal/privacy-policy.pt-BR.md (português). Inglês: docs/legal/privacy-policy.en.md.
  O texto de consentimento exibido no app é docs/legal/llm-consent.json (versão "3"), que deve permanecer consistente com este texto.
  Status: FINAL (versão 3, 15/11/2026). Todos os placeholders foram preenchidos.
-->

# Política de privacidade: geração de personagem com IA

Última atualização: 15/11/2026. Versão da política: 3.

Esta seção complementa a Política de Privacidade da Legnaris (https://legnaris.paulovalleriote.io/privacy). Ela descreve como o recurso opcional de geração de personagem com IA trata seus dados pessoais.

## 1. Controlador

O controlador dos seus dados pessoais é Paulo Valleriote, pessoa física, que pode ser contatado em paulovalleriotedev@gmail.com.

## 2. O que o recurso trata

Quando você pede à Legnaris para gerar um personagem, o texto do conceito que você digita é enviado a um modelo de IA. O resultado gerado é devolvido a você. A Legnaris armazena ambos apenas por um período curto (seção 5) e nunca os grava em logs.

## 3. Operadores

Para executar o recurso, usamos os seguintes operadores:

- **OpenRouter**, um gateway de modelos que recebe sua requisição e a encaminha. Pessoa jurídica: OpenRouter, Inc., Estados Unidos.
- **Provedores do modelo**, as empresas que executam o modelo DeepSeek V4.1 Flash, selecionadas pela OpenRouter para cada requisição. O provedor pode ser a API da DeepSeek ou outra empresa, localizada em países diferentes, inclusive na China. A OpenRouter publica a lista de provedores e suas localizações.

As requisições usam o roteamento padrão da OpenRouter, com alternativas entre provedores. O roteamento não é restrito por retenção de dados; portanto, um provedor que atenda a sua requisição pode reter prompts e resultados, ou usá-los em treinamento, conforme os termos próprios dele. A Legnaris não controla esses termos. Você pode consultá-los na documentação da OpenRouter e no site de cada provedor.

## 4. Transferência internacional

O tratamento ocorre fora do Brasil, o que pode incluir a China, conforme o provedor que atende a sua requisição. Trata-se de transferência internacional de dados pessoais nos termos do art. 33 da Lei Geral de Proteção de Dados (LGPD). Usamos como base legal o seu consentimento específico e destacado, dado no app antes da sua primeira geração. Não iniciamos nenhuma transferência sem esse consentimento.

## 5. Retenção

- **Na Legnaris:** o conceito e o resultado gerado são mantidos por até 24 horas e depois apagados automaticamente.
- **Nos operadores:** a OpenRouter e os provedores do modelo retêm ou usam prompts e resultados conforme os termos próprios deles, o que pode incluir retenção e uso para treinamento. A Legnaris não controla esses termos.
- **Na sua Ficha:** o conteúdo gerado que você mantém na sua própria Ficha permanece lá até você apagá-lo.

Sua exportação de dados não inclui os dados de geração, pois eles são apagados em até 24 horas.

## 6. Revogação do consentimento

Você pode retirar seu consentimento para o tratamento por IA a qualquer momento, na tela Conta (/account) ou pela API em DELETE /me/llm-consent. A retirada:

- apaga os dados de geração que a Legnaris ainda mantém sobre você, e
- interrompe qualquer nova transferência do seu conceito aos operadores.

A retirada não apaga o conteúdo já presente na sua Ficha. Você pode apagar esse conteúdo na própria Ficha. A retirada não afeta o restante da sua conta. Você pode dar o consentimento novamente a qualquer momento.

## 7. Seus direitos

Pela LGPD, você pode solicitar a confirmação do tratamento, acesso, correção, anonimização, portabilidade, eliminação, informação sobre as entidades com as quais compartilhamos seus dados e informação sobre a possibilidade de não fornecer consentimento e sobre as consequências dessa negativa. Envie suas solicitações para paulovalleriotedev@gmail.com.

## 8. Contato do encarregado de dados

Fundador da Legnaris (encarregado de proteção de dados), paulovalleriotedev@gmail.com.

## 9. Alterações

Quando os operadores, o modelo, a região de processamento ou o período de retenção mudarem, atualizaremos esta política e o texto de consentimento, e pediremos que você consinta novamente. As alterações são identificadas pela versão da política no início deste documento.
