<!--
  Atualização da política de privacidade: geração de personagem com LLM (LGN-228, ADR 0008).
  Local: docs/legal/privacy-policy.pt-BR.md (português). Inglês: docs/legal/privacy-policy.en.md.
  O texto de consentimento exibido no app é docs/legal/llm-consent.json (versão "1"), que deve permanecer consistente com este texto.
  Status: RASCUNHO, pendente de revisão jurídica. Cada token [COLCHETES] é um placeholder que o jurídico deve preencher; nenhum nome de pessoa jurídica ou endereço foi inventado aqui.
-->

# Política de privacidade: geração de personagem com IA

Última atualização: [DATA]. Versão da política: 1.

Esta seção complementa a Política de Privacidade da Legnaris [URL_POLITICA_PRINCIPAL]. Ela descreve como o recurso opcional de geração de personagem com IA trata seus dados pessoais.

## 1. Controlador

O controlador dos seus dados pessoais é [NOME_JURIDICO_CONTROLADOR], [ENDERECO_CONTROLADOR], [CNPJ_OU_EQUIVALENTE].

## 2. O que o recurso trata

Quando você pede à Legnaris para gerar um personagem, o texto do conceito que você digita é enviado a um modelo de IA. O resultado gerado é devolvido a você. A Legnaris armazena ambos apenas por um período curto (seção 5) e nunca os grava em logs.

## 3. Operadores

Para executar o recurso, usamos dois operadores:

- **OpenRouter**, um gateway de modelos que recebe sua requisição e a encaminha. Pessoa jurídica: [NOME_JURIDICO_OPENROUTER], [PAIS_OPENROUTER].
- **[SERVING_PROVIDER]**, o provedor que executa o modelo DeepSeek V4.1 Flash, selecionado pela OpenRouter para a sua requisição. Pessoa jurídica: [NOME_JURIDICO_SERVING_PROVIDER], [PAIS_SERVING_PROVIDER].

As requisições são configuradas para que a OpenRouter encaminhe somente a provedores com retenção zero de dados e sem uso dos seus dados para treinamento, e nunca à API própria da DeepSeek. A revisão jurídica deve confirmar esses termos com os contratos de tratamento de dados vigentes dos operadores.

## 4. Transferência internacional

O tratamento ocorre fora do Brasil, em [PAISES_DE_PROCESSAMENTO]. Trata-se de transferência internacional de dados pessoais nos termos do art. 33 da Lei Geral de Proteção de Dados (LGPD). Usamos como base legal o seu consentimento específico e destacado, dado no app antes da sua primeira geração. Não iniciamos nenhuma transferência sem esse consentimento.

## 5. Retenção

- **Na Legnaris:** o conceito e o resultado gerado são mantidos por até 24 horas e depois apagados automaticamente.
- **Nos operadores:** os operadores estão configurados para não reter prompts nem resultados e para não usá-los em treinamento. [RETENCAO_NOS_OPERADORES_CONFIRMADA_PELO_JURIDICO]
- **Na sua Ficha:** o conteúdo gerado que você mantém na sua própria Ficha permanece lá até você apagá-lo.

Sua exportação de dados não inclui os dados de geração, pois eles são apagados em até 24 horas.

## 6. Revogação do consentimento

Você pode retirar seu consentimento para o tratamento por IA a qualquer momento, em [CAMINHO_TELA_REVOGACAO] ou pela API em [CAMINHO_API_REVOGACAO]. A retirada:

- apaga os dados de geração que a Legnaris ainda mantém sobre você, e
- interrompe qualquer nova transferência do seu conceito aos operadores.

A retirada não apaga o conteúdo já presente na sua Ficha. Você pode apagar esse conteúdo na própria Ficha. A retirada não afeta o restante da sua conta. Você pode dar o consentimento novamente a qualquer momento.

## 7. Seus direitos

Pela LGPD, você pode solicitar a confirmação do tratamento, acesso, correção, anonimização, portabilidade, eliminação, informação sobre as entidades com as quais compartilhamos seus dados e informação sobre a possibilidade de não fornecer consentimento e sobre as consequências dessa negativa. Envie suas solicitações para [EMAIL_CONTATO_PRIVACIDADE].

## 8. Contato do encarregado de dados

[NOME_OU_FUNCAO_DO_ENCARREGADO], [EMAIL_DO_ENCARREGADO].

## 9. Alterações

Quando os operadores, o modelo, a região de processamento ou o período de retenção mudarem, atualizaremos esta política e o texto de consentimento, e pediremos que você consinta novamente. As alterações são identificadas pela versão da política no início deste documento.
