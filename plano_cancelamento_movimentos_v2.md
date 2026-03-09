**PLANO DE IMPLEMENTAÇÃO**

**Cancelamento de Movimentos --- Nova Arquitetura**

blc-pblc-mov-actions \| Temporal Workflows \| Março 2026

> **⚠ Documento confidencial --- uso interno**
>
> **1. Visão Geral**

Este documento detalha todas as alterações necessárias para implementar
o Cancelamento de Movimentos na nova arquitetura Temporal do
blc-pblc-mov-actions, incluindo os trechos de código exatos a criar ou
modificar em cada arquivo.

**Escopo desta sprint**

-   Cancelamento de compra e venda (código 0152)

-   Comando único (mesmo radical) e comando duplo (radicais diferentes)

-   Sem financeiro (sem modalidade) e com financeiro (modalidade bruta)

-   Flag de cancelamento no movimento original

-   Reversão de custódia

**Fora do escopo (próxima sprint)**

-   Validação completa de cadeia de intermediação (CA40-CA42)

-   Cancelamento de TSF

> **2. Resumo de Arquivos**

Legenda de repos: \[mov-actions\] = blc-pblc-mov-actions \| \[lib-mov\]
= blc-pblc-mov-workflow-movements \| \[matching\] =
blc-pblc-mov-workflow-matching

  ---------------------------------------------------------------------------------------------------------------
  **Arquivo**                                      **Tipo**                **Ação**
  ------------------------------------------------ ----------------------- --------------------------------------
  \[mov-actions\] CCBMovementDTO                   **Existente**           Adicionar campo codControleMovimento
                                                                           (String) --- campo de entrada da API

  \[lib-mov\] MovementOrderDTO                     **Existente**           Adicionar campo codControleMovimento
                                                                           (String) --- exige bump de versão da
                                                                           lib

  \[mov-actions\]                                  **Existente**           Alterar geração do correlationId ---
  EntryOrderWorkflowStarterAdapterImpl                                     incluir codControleMovimento quando
                                                                           operationType começa com \'01\'

  \[mov-actions\] OrderEntryWorkflowAdapterImpl    **Existente**           Adicionar branch após validações: se
                                                                           isCancelamento(), desviar para
                                                                           CancelamentoWorkflow (child)

  \[mov-actions\]                                  **Existente**           Adicionar método
  MovementOperationRegistrationActivitiesAdapter                           saveWorkflowId(movementId, workflowId)
                                                                           na interface e impl

  \[mov-actions\] MovementEntity                   **Existente**           Adicionar campos temporal_workflow_id
                                                                           e cancelamento_solicitado (migration
                                                                           necessária)

  \[mov-actions\] UpdateEntityAdapterImpl          **Existente**           Reutilizado via
                                                                           updateEntity(MovementOrderDTO) --- sem
                                                                           alteração na assinatura, mas
                                                                           AtualizaStatusActivity monta o DTO
                                                                           corretamente

  \[mov-actions\] LogicalValidationsEnum           **Existente**           Adicionar
                                                                           VALIDACAO_MOVIMENTO_ORIGINAL_EXISTE,
                                                                           VALIDACAO_DATA_OPERACAO_D0,
                                                                           VALIDACAO_STATUS_CANCELAVEL,
                                                                           VALIDACAO_CODIGO_CANCELAMENTO

  \[mov-actions\] GeneralWorkflowConfiguration     **Existente**           Adicionar cancelamentoWorkflowQueue,
                                                                           cancelamentoEnabled,
                                                                           cancelamentoValidations

  \[mov-actions\] QueueConfig                      **Existente**           Adicionar CANCELLATION_WORKFLOW_QUEUE
                                                                           e CANCELLATION_ACTIVITY_QUEUE

  \[lib-mov\] MovementWorkflow (interface)         **Existente**           Adicionar \@SignalMethod
                                                                           receberFlagCancelamento() e
                                                                           retornarFluxoOriginal(); \@QueryMethod
                                                                           isCancelamentoPendente()

  \[lib-mov\] MovementWorkflowImpl                 **Existente**           Adicionar campos de estado + handlers
                                                                           dos Signals + Workflow.await() no
                                                                           início de handleFlowWithSettlement
                                                                           (antes linha 147) e
                                                                           handleFlowWithoutSettlement (antes
                                                                           linha 180)

  \[lib-mov\] WorkflowStatus enum                  **Existente**           Adicionar CANCELLATION_PENDING

  \[matching\] MatchingWorkflow                    **Existente**           Adicionar branch no executeMatching:
                                                                           se operationType começa com \'01\',
                                                                           usar codControleMovimento como chave
                                                                           em vez do fluxo normal

  \[mov-actions\] CancelamentoWorkflow (interface) **Novo**                Interface Temporal:
                                                                           executeCancelamento(),
                                                                           confirmarCancelamentoContraparte(),
                                                                           receberRetornoLiquidacao(),
                                                                           receberRetornoUnlock()

  \[mov-actions\] CancelamentoWorkflowImpl         **Novo**                Orquestra: recupera original → valida
                                                                           → duplo comando → flag → cadeia →
                                                                           carteira/liquidação

  \[mov-actions\]                                  **Novo**                Busca MovementEntity por
  RecuperaMovimentoOriginalActivity                                        movementControlCode
                                                                           (codControleMovimento)

  \[mov-actions\] ValidaCancelamentoActivity       **Novo**                Validações CA19-CA22, CA36 --- mesmo
                                                                           dia, status cancelável, código 01XX,
                                                                           grade

  \[mov-actions\] FlagCancelamentoActivity         **Novo**                Seta cancelamento_solicitado=true no
                                                                           MovementEntity original + envia Signal
                                                                           se workflow ainda ativo

  \[mov-actions\] ReversaoCarteiraActivity         **Novo**                Monta payload inverso e envia ao
                                                                           tópico de carteira

  \[mov-actions\]                                  **Novo**                Atualiza status via
  AtualizaStatusCancelamentoActivity                                       updateEntity(DTO) + notifica tópico de
                                                                           saída via MessageAdapter

  \[mov-actions\]                                  **Novo**                Objeto de configuração parametrizado
  CancelamentoWorkflowConfiguration                                        via hot config
  ---------------------------------------------------------------------------------------------------------------

> **3. Alterações em Arquivos Existentes**
>
> **3.1 QueueConfig**

**Por que mudar**

O novo CancelamentoWorkflow precisa de sua própria task queue no
Temporal, seguindo o padrão do projeto onde cada domínio tem sua fila
isolada.

**Alteração**

> // Arquivo: configuration/QueueConfig.java
>
> // Adicionar ao final da classe:
>
> public static final String CANCELLATION_WORKFLOW_QUEUE =
> \"cancellation-workflow-queue\";
>
> public static final String CANCELLATION_ACTIVITY_QUEUE =
> \"cancellation-activity-queue\";
>
> **3.2 GeneralWorkflowConfiguration**

**Por que mudar**

Todos os parâmetros do cancelamento precisam ser configuráveis via hot
config, sem redeploy. Seguindo o padrão já existente
(getMatchWorkflowQueue, getMovementsWorkflowQueue, etc.).

**Campos a adicionar**

> // Arquivo: domain/GeneralWorkflowConfiguration.java
>
> // Adicionar os seguintes campos:
>
> // Queue do novo workflow
>
> private String cancelamentoWorkflowQueue;
>
> // Habilita/desabilita o processamento de cancelamentos
>
> private Boolean cancelamentoEnabled;
>
> // Lista de validações lógicas específicas do cancelamento
>
> private List\<LogicalValidationsEnum\> cancelamentoValidations;
>
> // Getters/Setters seguindo o padrão existente
>
> public String getCancelamentoWorkflowQueue() {
>
> return cancelamentoWorkflowQueue;
>
> }
>
> public Boolean getCancelamentoEnabled() {
>
> return cancelamentoEnabled != null && cancelamentoEnabled;
>
> }
>
> **3.3 LogicalValidationsEnum**

**Por que mudar**

As validações do cancelamento são distintas das validações do lançamento
normal. Elas precisam ser identificadas individualmente para que o hot
config possa habilitar/desabilitar cada uma.

**Valores a adicionar**

> // Arquivo: domain/enums/LogicalValidationsEnum.java
>
> // Adicionar os seguintes valores ao enum:
>
> // Verifica se o movimento original existe pelo codControleMovimento
> (CA20)
>
> VALIDACAO_MOVIMENTO_ORIGINAL_EXISTE,
>
> // Verifica se a data do movimento original é D0 (CA21)
>
> VALIDACAO_DATA_OPERACAO_D0,
>
> // Verifica se o status do movimento original é cancelável (CA22/CA36)
>
> VALIDACAO_STATUS_CANCELAVEL,
>
> // Verifica se o código do movimento de cancelamento é 01XX (CA51 doc)
>
> VALIDACAO_CODIGO_CANCELAMENTO,
>
> // Verifica grade para cancelamento (CA19)
>
> VALIDACAO_GRADE_CANCELAMENTO,
>
> // Verifica se já existe liquidação em andamento (CA37.1)
>
> VALIDACAO_PENDENTE_LIQUIDACAO,
>
> **3.4 MovementEntity +
> MovementOperationRegistrationActivitiesAdapter**

**Por que mudar**

Precisamos de dois campos novos no MovementEntity: temporal_workflow_id
(para o CancelamentoWorkflow enviar Signal ao workflow original) e
cancelamento_solicitado (flag CA50). Também precisamos do método
saveWorkflowId() na interface e impl do adapter.

**Campos a adicionar em MovementEntity**

> // Arquivo: adapters/database/entity/MovementEntity.java
>
> // workflowId do Temporal --- para envio de Signals (CA50/CA51/CA52)
>
> \@Column(name = \"temporal_workflow_id\")
>
> private String temporalWorkflowId;
>
> // Flag de cancelamento (CA50) --- true quando CancelamentoWorkflow
> solicitou pausa
>
> \@Column(name = \"cancelamento_solicitado\", nullable = false)
>
> \@Builder.Default
>
> private Boolean cancelamentoSolicitado = false;
>
> ⚠ Migration necessária: adicionar colunas temporal_workflow_id
> (VARCHAR 255) e
>
> cancelamento_solicitado (BOOLEAN DEFAULT FALSE) na tabela de
> movimentos.

**Método saveWorkflowId --- interface
MovementOperationRegistrationActivitiesAdapter**

> MovementEntity.movementId é Long (PK gerada por IDENTITY), não String.
>
> CasamentoMovimentoDto.movimentoId é String.
>
> Usar movementControlCode como chave de lookup, pois é o elo entre os
> domínios.
>
> Alternativa: verificar se movimentoId do CasamentoDto corresponde ao
> movementCode
>
> (COD_MOVIMENTO, String, length=100) em MovementEntity.
>
> // Adicionar à interface:
>
> // Persiste o workflowId do Temporal no movimento para uso em Signals
> futuros
>
> // movementControlCode = MovementEntity.movementControlCode
> (COD_CONTROLE_MOVIMENTO)
>
> void saveWorkflowId(String movementControlCode, String workflowId);

**Método saveWorkflowId --- impl
MovementOperationRegistrationActivitiesAdapterImpl**

> // Adicionar após o método setChainCodeToMovement (linha \~145):
>
> \@Override
>
> public void saveWorkflowId(String movementControlCode, String
> workflowId) {
>
> movementRepository.findByMovementControlCode(movementControlCode)
>
> .ifPresent(entity -\> {
>
> entity.setTemporalWorkflowId(workflowId);
>
> movementRepository.save(entity);
>
> log.info(\"\[SaveWorkflowId\] workflowId={} para
> movementControlCode={}\",
>
> workflowId, movementControlCode);
>
> });
>
> }
>
> // Adicionar ao MovementRepository (reutilizado também em
> RecuperaMovimentoOriginalActivity):
>
> Optional\<MovementEntity\> findByMovementControlCode(String
> movementControlCode);

**Onde chamar saveWorkflowId --- OrderEntryWorkflowAdapterImpl**

Logo após saveMovement(\...). O movementControlCode já está no
workflowMovementDTO (gerado no createMovementEntity, linha 166):

> // Após
> movementOperationRegistrationActivitiesAdapter.saveMovement(\...)
>
> String currentWorkflowId = Workflow.getInfo().getWorkflowId();
>
> // workflowMovementDTO.getMovementOrderDTO() tem o
> codControleMovimento propagado
>
> // desde EntryOrderWorkflowStarterAdapterImpl (seção 3.6)
>
> movementOperationRegistrationActivitiesAdapter
>
> .saveWorkflowId(
>
> workflowMovementDTO.getMovementOrderDTO().getCodControleMovimento(),
> // ⚠ requer bump da lib (seção 3.5) --- campo não existe na versão
> atual de MovementOrderDTO
>
> currentWorkflowId);
>
> **3.5 Novos campos nos DTOs --- CCBMovementDTO e MovementOrderDTO**
>
> ATENÇÃO --- Ambos os DTOs não possuem o campo codControleMovimento.
>
> CCBMovementDTO está em: blc-pblc-mov-actions
> (domain/CCBMovementDTO.java)
>
> MovementOrderDTO está em: blc-pblc-lib-mov (lib compilada ---
> blc-pblc-lib-mov-1.1.17.jar)
>
> MovementOrderDTO exige bump de versão da lib antes de ser usada no
> mov-actions.

**CCBMovementDTO --- adicionar campo (blc-pblc-mov-actions)**

> // Arquivo: domain/CCBMovementDTO.java
>
> // Campos existentes: tickerSymbol, partyRoleTypeCode,
> otcPartyAccountCode,
>
> // otcCounterpartyAccountCode, operationQuantity, modalityCode,
> selfNumber,
>
> // unitPriceValue, deleteIndicator, List\<InvestorDTO\> investors
>
> // ADICIONAR:
>
> \@JsonProperty(\"codControleMovimento\")
>
> private String codControleMovimento;
>
> // Getter/Setter gerado pelo \@Data do Lombok já existente na classe

**MovementOrderDTO --- adicionar campo (blc-pblc-lib-mov)**

> // Arquivo: blc-pblc-lib-mov / MovementOrderDTO.java
>
> // Campos existentes: movementOrderId, additionalData, clients,
> financialInstrument,
>
> // party, counterParty, assetQuantity, unitPriceValue,
> totalOperationValue,
>
> // settlementModalityTypeCode, operationTypeCode, partyRole,
> partyRoleType,
>
> // selfNumber, associationNumber, operationDate, settlementDate
>
> // ADICIONAR:
>
> \@JsonProperty(\"codControleMovimento\")
>
> private String codControleMovimento;
>
> // Getter/Setter (lib não usa Lombok --- adicionar manualmente se
> necessário)
>
> public String getCodControleMovimento() { return codControleMovimento;
> }
>
> public void setCodControleMovimento(String v) {
> this.codControleMovimento = v; }
>
> Sequência de deploy para este campo:
>
> 1\. Adicionar codControleMovimento no MovementOrderDTO da lib
> (blc-pblc-lib-mov)
>
> 2\. Bump de versão: ex. 1.1.17 → 1.1.18
>
> 3\. Atualizar dependência da lib no pom.xml do blc-pblc-mov-actions
>
> 4\. Adicionar o campo no CCBMovementDTO do mov-actions
>
> 5\. Mapear o campo no EntryOrderWorkflowStarterAdapterImpl (ver seção
> 3.6)
>
> **3.6 EntryOrderWorkflowStarterAdapterImpl**

**Por que mudar**

O correlationId (chave de idempotência) hoje é gerado com:
movementTypeCode + selfNumber + partyAccount + counterpartyAccount +
date + ticker. Para cancelamento, o codControleMovimento do movimento
original deve ser incluído como discriminador único, pois o mesmo par de
contas pode ter múltiplos cancelamentos em dias diferentes.

**Alteração no método beginWorkflowProcess**

> // Arquivo:
> adapters/starters/impl/EntryOrderWorkflowStarterAdapterImpl.java
>
> private MovementIdentifierDTO beginWorkflowProcess(
>
> CCBMovementDTO requestMovementDTO,
>
> String requestTickerSymbolTypeCode,
>
> String requestMovementType,
>
> String userCode) throws JsonProcessingException {
>
> // \... código existente até a geração do correlationId \...
>
> // ALTERAÇÃO: incluir codControleMovimento se for cancelamento
>
> boolean isCancelamento = requestMovementType != null
>
> && requestMovementType.startsWith(\"01\");
>
> String correlationId;
>
> if (isCancelamento) {
>
> // Para cancelamento, codControleMovimento do original é a chave
> principal
>
> String codControleMovimento =
> requestMovementDTO.getCodControleMovimento();
>
> if (codControleMovimento == null \|\| codControleMovimento.isBlank())
> {
>
> throw new IllegalArgumentException(
>
> \"codControleMovimento é obrigatório para movimentos de
> cancelamento\");
>
> }
>
> correlationId = launchIdempotenceHelper.generateOrderEntryId(
>
> movementTypeEnum.getMovementTypeCode().toString(),
>
> selfNumber,
>
> partyAccountNumber,
>
> counterpartyAccountNumber,
>
> localDate.format(formatter),
>
> requestTickerSymbolTypeCode,
>
> codControleMovimento // discriminador adicional
>
> );
>
> } else {
>
> // Lógica original inalterada
>
> correlationId = launchIdempotenceHelper.generateOrderEntryId(
>
> movementTypeEnum.getMovementTypeCode().toString(),
>
> selfNumber,
>
> partyAccountNumber,
>
> counterpartyAccountNumber,
>
> localDate.format(formatter),
>
> requestTickerSymbolTypeCode,
>
> requestMovementDTO.getTickerSymbol()
>
> );
>
> }
>
> // NOVO: propagar codControleMovimento para o MovementOrderDTO
>
> // (campo adicionado na lib --- ver seção 3.5)
>
> movementOrderDTO.setCodControleMovimento(requestMovementDTO.getCodControleMovimento());
>
> // \... resto do código existente \...
>
> }
>
> **3.6 OrderEntryWorkflowAdapterImpl (arquivo mais impactado)**

**Por que mudar**

Este é o workflow principal. Após as validações física e lógica, ele
precisa identificar se o movimento é um cancelamento e desviar o fluxo
para o CancelamentoWorkflow em vez de seguir para o CasamentoWorkflow
normal.

**Passo 1 --- Persistir o workflowId ao salvar o movimento**

Hoje na linha \~367 o movementOrderId é setado. Precisamos também
guardar o workflowId do Temporal para que o CancelamentoWorkflow possa
enviar Signals ao workflow original.

> // Após linha 367 (movementOrderDTO.setMovementOrderId(\...)):
>
> // Persiste o workflowId do Temporal para uso em Signals futuros
>
> String currentWorkflowId = Workflow.getInfo().getWorkflowId();
>
> movementOperationRegistrationActivitiesAdapter
>
> .saveWorkflowId(movementOrderDTO.getMovementOrderId(),
> currentWorkflowId);

**Passo 2 --- Branch de cancelamento após validação lógica**

Após a linha \~395 (statusWorkflow.changeStatus(statusUpdate) com
SUCCESS_STATUS), adicionar a detecção de cancelamento:

> // Após statusWorkflow.changeStatus(statusUpdate) com SUCCESS_STATUS
> (\~linha 398):
>
> // NOVO: Branch de cancelamento
>
> if (isCancelamento(movementOrderDTO)) {
>
> log.info(\"Movimento identificado como CANCELAMENTO. Desviando para
> CancelamentoWorkflow.\");
>
> CancelamentoInterface cancelamentoWorkflow =
> Workflow.newChildWorkflowStub(
>
> CancelamentoInterface.class,
>
> ChildWorkflowOptions.newBuilder()
>
> .setTaskQueue(generalWorkflowConfig.getResult()
>
> .getCancelamentoWorkflowQueue())
>
> .setParentClosePolicy(ParentClosePolicy.PARENT_CLOSE_POLICY_ABANDON)
>
> .setWorkflowId(\"cancelamento-\" +
> movementOrderDTO.getMovementOrderId())
>
> .build());
>
> cancelamentoWorkflow.executeCancelamento(movementOrderDTO);
>
> return; // OrderEntryWorkflow encerra aqui para cancelamentos
>
> }
>
> // Fluxo original de casamento continua abaixo para movimentos
> normais\...
>
> CasamentoInterface matchingWorkflow =
> Workflow.newChildWorkflowStub(\...);

**Passo 3 --- Método auxiliar isCancelamento**

> // Adicionar método privado na classe:
>
> private static boolean isCancelamento(MovementOrderDTO
> movementOrderDTO) {
>
> String operationType = movementOrderDTO.getOperationTypeCode();
>
> return operationType != null && operationType.startsWith(\"01\");
>
> }
>
> **3.7 MovementWorkflow --- Signal para flag de cancelamento**
>
> ⚠ REPO: blc-pblc-mov-workflow-movements
>
> Arquivo:
> src/main/java/br/com/b3/workflow_movements/workflow/MovementWorkflow.java
>
> Impl:
> src/main/java/br/com/b3/workflow_movements/workflow/MovementWorkflowImpl.java
>
> A lib blc-pblc-lib-mov é gerada a partir deste repo --- alterações
> aqui
>
> exigem novo build + bump de versão da lib antes do deploy do
> mov-actions.

**Por que mudar**

CA50/CA51/CA52 exigem que o movimento original pause seu fluxo ao
receber a flag de cancelamento. O fluxo real do MovementWorkflowImpl tem
dois pontos críticos onde a pausa deve ocorrer: no início de
handleFlowWithSettlement (antes de executeSettlementWorkflow, linha 148)
e no início de handleFlowWithoutSettlement (antes de
executePortfolioWorkflowByMovementType, linha 180).

**1. Alteração na interface MovementWorkflow**

Repo: blc-pblc-mov-workflow-movements --- arquivo MovementWorkflow.java

> // Interface atual (decompilada do .class):
>
> // \@WorkflowMethod void startWorkflow(String var1);
>
> // \@QueryMethod String getWorkflowStatus();
>
> // \@QueryMethod String getLastError();
>
> // ADICIONAR os seguintes métodos:
>
> // Signal para pausar o fluxo quando cancelamento for solicitado
> (CA50)
>
> \@SignalMethod
>
> void receberFlagCancelamento();
>
> // Signal para retomar o fluxo quando cancelamento falhar ou for
> abortado
>
> \@SignalMethod
>
> void retornarFluxoOriginal();
>
> // Query para o CancelamentoWorkflow consultar o estado sem Signal
>
> \@QueryMethod
>
> boolean isCancelamentoPendente();

**2. Alteração em MovementWorkflowImpl --- campos de estado**

Adicionar logo após os campos existentes (linha \~43, após
MovementDataActivity dataActivity):

> // NOVO: estado de controle do cancelamento (CA50/CA51/CA52)
>
> private volatile boolean cancelamentoPendente = false;
>
> private volatile boolean retornarFluxo = false;

**3. Implementar os handlers dos Signals**

Adicionar após o método getDataActivity() (linha \~68):

> \@Override
>
> public void receberFlagCancelamento() {
>
> logger.info(\"\[MovementWorkflow\] Signal receberFlagCancelamento
> recebido\");
>
> stateManager.updateStatus(WorkflowStatus.CANCELLATION_PENDING); //
> novo enum
>
> this.cancelamentoPendente = true;
>
> }
>
> \@Override
>
> public void retornarFluxoOriginal() {
>
> logger.info(\"\[MovementWorkflow\] Signal retornarFluxoOriginal
> recebido\");
>
> stateManager.updateStatus(WorkflowStatus.RUNNING);
>
> this.cancelamentoPendente = false;
>
> this.retornarFluxo = true;
>
> }
>
> \@Override
>
> public boolean isCancelamentoPendente() {
>
> return cancelamentoPendente;
>
> }

**4. Inserir Workflow.await em handleFlowWithSettlement**

Inserir imediatamente ANTES da linha 147
(stateManager.updateStatus(EXECUTING_SETTLEMENT)):

> // handleFlowWithSettlement --- linha 136 do arquivo original
>
> private void handleFlowWithSettlement(String idCadeia,
>
> LancamentoEntity lancamento,
>
> MovimentarCarteiraWorkflow portfolioWorkflow) throws
> B3WorkflowException {
>
> // NOVO: Pausa se cancelamento foi solicitado antes de entrar na
> liquidação
>
> // Se o CancelamentoWorkflow enviar retornarFluxoOriginal(), o fluxo
> continua
>
> if (cancelamentoPendente) {
>
> logger.info(\"\[MovementWorkflow\] Pausando antes de settlement ---
> cancelamento pendente\");
>
> Workflow.await(() -\> !cancelamentoPendente \|\| retornarFluxo);
>
> logger.info(\"\[MovementWorkflow\] Retomando após resolução do
> cancelamento\");
>
> }
>
> try {
>
> // ── código original a partir daqui (linha 146) ──
>
> stateManager.updateStatus(WorkflowStatus.EXECUTING_SETTLEMENT);
>
> workflowExecutor.executeSettlementWorkflow(settlementWorkflow,
> idCadeia);
>
> executePortfolioWorkflowByMovementType(portfolioWorkflow, idCadeia,
> lancamento);
>
> } catch (ApplicationFailure e) {
>
> // \... catch existente inalterado \...
>
> }
>
> }

**5. Inserir Workflow.await em handleFlowWithoutSettlement**

Inserir imediatamente ANTES da linha 180
(executePortfolioWorkflowByMovementType):

> // handleFlowWithoutSettlement --- linha 178 do arquivo original
>
> private void handleFlowWithoutSettlement(String idCadeia,
>
> LancamentoEntity lancamento,
>
> MovimentarCarteiraWorkflow portfolioWorkflow) {
>
> // NOVO: Pausa se cancelamento foi solicitado antes de mover carteira
>
> if (cancelamentoPendente) {
>
> logger.info(\"\[MovementWorkflow\] Pausando antes de portfolio ---
> cancelamento pendente\");
>
> Workflow.await(() -\> !cancelamentoPendente \|\| retornarFluxo);
>
> logger.info(\"\[MovementWorkflow\] Retomando após resolução do
> cancelamento\");
>
> }
>
> // ── código original inalterado (linha 180) ──
>
> executePortfolioWorkflowByMovementType(portfolioWorkflow, idCadeia,
> lancamento);
>
> }

**6. Adicionar CANCELLATION_PENDING ao WorkflowStatus enum**

Repo: blc-pblc-mov-workflow-movements --- arquivo
enums/WorkflowStatus.java

> // Adicionar ao enum WorkflowStatus:
>
> CANCELLATION_PENDING, // Movimento pausado aguardando resolução do
> cancelamento
>
> ATENÇÃO --- Sequência de deploy obrigatória (5 repos):
>
> 1\. blc-pblc-lib-mov: adicionar codControleMovimento no
> MovementOrderDTO
>
> → bump de versão: ex. 1.1.17 → 1.1.18
>
> 2\. blc-pblc-mov-workflow-movements: Signals + Workflow.await +
> WorkflowStatus enum
>
> → lib-mov já atualizada (passo 1) deve estar no pom.xml deste repo
>
> → bump de versão desta lib se for necessário (ex. 2.x.x → 2.x.x+1)
>
> 3\. blc-pblc-mov-workflow-matching: branch cancelamento no
> MatchingWorkflow
>
> → verificar se precisa bump de versão ou deploy direto
>
> 4\. blc-pblc-mov-actions: todos os demais itens deste plano
>
> → atualizar dependências para versões dos passos 1 e 2
>
> 5\. Migrations de banco (temporal_workflow_id e
> cancelamento_solicitado)
>
> → executar ANTES do deploy do mov-actions (passo 4)
>
> **3.8 MatchingWorkflow --- Chave de casamento para cancelamento**
>
> ⚠ REPO: blc-pblc-mov-workflow-matching
>
> Arquivo:
> src/main/java/br/com/b3/workflowMatching/adapters/workflowV2/MatchingWorkflow.java
>
> Este repo também é uma lib/serviço separado --- verificar se gera
> artefato consumido
>
> pelo mov-actions ou se é um serviço independente com sua própria fila
> Temporal.

**Por que mudar**

O executeMatching atual (linha 49-134) usa checkSingleOrDoubleCommand,
processMatching e buildResultWithMatch como chaves de casamento. Para
cancelamento, o codControleMovimento do movimento original deve ser a
chave, não o par de contas/ativo padrão.

**Alteração em executeMatching --- adicionar branch no início**

Inserir logo após a linha 49 (início do executeMatching), antes da
lógica de roleEnum:

> // Arquivo: MatchingWorkflow.java --- repo
> blc-pblc-mov-workflow-matching
>
> // Adicionar no início do método executeMatching:
>
> \@Override
>
> public CasamentoResultDto executeMatching(MovementOrderDTO orderDTO) {
>
> ActivityReturnDTO\<CasamentoResultDto\> processReturn;
>
> // NOVO: Branch de cancelamento --- usa codControleMovimento como
> chave
>
> boolean isCancelamento = orderDTO.getOperationTypeCode() != null
>
> && orderDTO.getOperationTypeCode().startsWith(\"01\");
>
> if (isCancelamento) {
>
> // Para cancelamento, o casamento é feito pelo codControleMovimento
>
> // do movimento original --- os dois lançamentos de cancelamento
>
> // (parte e contraparte) se casam por este código
>
> processReturn = matchingInterface.processMatchingByCodControle(
>
> orderDTO, orderDTO.getCodControleMovimento());
>
> if (processReturn.getMessageError() != null) {
>
> throw ApplicationFailure.newFailure(
>
> processReturn.getMessageError()
>
> .getErrorDetails().getFirst().getErrorTitle(),
>
> processReturn.getMessageError()
>
> .getErrorDetails().getFirst().getErrorDescription());
>
> }
>
> return processReturn.getResult();
>
> }
>
> // ── fluxo original inalterado a partir daqui (linha 52) ──
>
> RoleEnum roleEnum = RoleEnum.getFromRoleName(orderDTO.getPartyRole())
>
> .orElseThrow(() -\> ApplicationFailure.newFailure(\...));
>
> // \... resto do código existente \...
>
> }

**Novo método na matchingInterface --- processMatchingByCodControle**

Adicionar à interface matchingInterface (activity) que MatchingWorkflow
já usa:

> // Na interface matchingInterface (activity do repo
> blc-pblc-mov-workflow-matching):
>
> ActivityReturnDTO\<CasamentoResultDto\> processMatchingByCodControle(
>
> MovementOrderDTO orderDTO,
>
> String codControleMovimento);
>
> // Impl: busca no banco os dois lançamentos de cancelamento pelo
>
> // codControleMovimento e os casa, retornando CasamentoResultDto com
>
> // isCadeiaFechada() = true quando ambos os lados estiverem presentes.
>
> **4. Novos Arquivos**
>
> **4.1 CancelamentoWorkflow --- Interface**

**Localização**

workflows/cancellation/CancelamentoWorkflow.java

> package br.com.b3.pblc.workflows.cancellation;
>
> import io.temporal.workflow.WorkflowInterface;
>
> import io.temporal.workflow.WorkflowMethod;
>
> import io.temporal.workflow.SignalMethod;
>
> import br.com.b3.pblc.domain.MovementOrderDTO;
>
> \@WorkflowInterface
>
> public interface CancelamentoWorkflow {
>
> \@WorkflowMethod
>
> void executeCancelamento(MovementOrderDTO cancelamentoDTO);
>
> // Signal recebido da contraparte no duplo comando (CA28)
>
> \@SignalMethod
>
> void confirmarCancelamentoContraparte(MovementOrderDTO
> contraparteDTO);
>
> // Signal recebido do WorkflowLiquidacao com resultado (CA37.2)
>
> \@SignalMethod
>
> void receberRetornoLiquidacao(String statusLiquidacao);
>
> // Signal recebido do WorkflowCarteira após unlock (CA37.3)
>
> \@SignalMethod
>
> void receberRetornoUnlock(boolean sucesso);
>
> }
>
> **4.2 CancelamentoWorkflowImpl --- Implementação completa**

**Localização**

workflows/cancellation/CancelamentoWorkflowImpl.java

**Estrutura geral**

> package br.com.b3.pblc.workflows.cancellation;
>
> \@WorkflowImpl
>
> public class CancelamentoWorkflowImpl implements CancelamentoWorkflow
> {
>
> // ── Activity stubs ──
>
> private final RecuperaMovimentoOriginalActivity recuperaMovimento =
>
> Workflow.newActivityStub(RecuperaMovimentoOriginalActivity.class,
>
> ActivityOptions.newBuilder()
>
> .setTaskQueue(CANCELLATION_ACTIVITY_QUEUE)
>
> .setStartToCloseTimeout(Duration.ofSeconds(30))
>
> .build());
>
> private final ValidaCancelamentoActivity validaCancelamento =
>
> Workflow.newActivityStub(ValidaCancelamentoActivity.class,
>
> ActivityOptions.newBuilder()
>
> .setTaskQueue(CANCELLATION_ACTIVITY_QUEUE)
>
> .setStartToCloseTimeout(Duration.ofSeconds(30))
>
> .setRetryOptions(RetryOptions.newBuilder()
>
> .setMaximumAttempts(3).build())
>
> .build());
>
> private final FlagCancelamentoActivity flagCancelamento =
>
> Workflow.newActivityStub(FlagCancelamentoActivity.class, \...);
>
> private final ReversaoCarteiraActivity reversaoCarteira =
>
> Workflow.newActivityStub(ReversaoCarteiraActivity.class, \...);
>
> private final AtualizaStatusCancelamentoActivity atualizaStatus =
>
> Workflow.newActivityStub(AtualizaStatusCancelamentoActivity.class,
> \...);
>
> private final MessageAdapter message =
>
> Workflow.newActivityStub(MessageAdapter.class, \...);
>
> // ── Estado interno (Signals) ──
>
> private boolean contraparteConfirmou = false;
>
> private MovementOrderDTO contraparteDTO = null;
>
> private String statusLiquidacao = null;
>
> private Boolean unlockSucesso = null;
>
> // \... implementação abaixo \...
>
> }

**executeWorkflow --- fluxo principal**

> \@Override
>
> public void executeCancelamento(MovementOrderDTO cancelamentoDTO) {
>
> log.info(\"\[CancelamentoWorkflow\] Iniciando para movOrder={}\",
>
> cancelamentoDTO.getMovementOrderId());
>
> // ── PASSO 1: Recuperar movimento original ──
>
> ActivityReturnDTO\<MovementOrderDTO\> originalResult =
>
> recuperaMovimento.recuperar(cancelamentoDTO.getCodControleMovimento());
>
> if (isError(originalResult)) {
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"E1\",
>
> originalResult.getMessageError());
>
> return;
>
> }
>
> MovementOrderDTO movimentoOriginal = originalResult.getResult();
>
> // ── PASSO 2: Validações do cancelamento ──
>
> ActivityReturnDTO\<Void\> validacaoResult =
>
> validaCancelamento.validar(cancelamentoDTO, movimentoOriginal);
>
> if (isError(validacaoResult)) {
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"E1\",
>
> validacaoResult.getMessageError());
>
> return;
>
> }
>
> // ── PASSO 3: Duplo comando --- aguardar contraparte (CA23/CA26) ──
>
> boolean isComandoDuplo = isComandoDuplo(movimentoOriginal);
>
> if (isComandoDuplo) {
>
> log.info(\"Duplo comando: aguardando confirmação da contraparte\");
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO,
>
> resolveStatusPendente(movimentoOriginal), null);
>
> // Aguarda signal da contraparte (sem timeout --- negócio define)
>
> Workflow.await(() -\> contraparteConfirmou);
>
> // Revalida após confirmação (CA24 --- contraparte pode cancelar
> antes)
>
> ActivityReturnDTO\<Void\> revalidacao =
>
> validaCancelamento.revalidarAposContraparte(
>
> cancelamentoDTO, movimentoOriginal);
>
> if (isError(revalidacao)) {
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"E1\",
>
> revalidacao.getMessageError());
>
> return;
>
> }
>
> }
>
> // ── PASSO 4: Setar flag de cancelamento no movimento original (CA50)
> ──
>
> flagCancelamento.setarFlag(movimentoOriginal.getMovementOrderId(),
>
> cancelamentoDTO.getMovementOrderId());
>
> // Enviar Signal ao workflow original se ainda estiver rodando
>
> String originalWorkflowId = movimentoOriginal.getTemporalWorkflowId();
>
> if (originalWorkflowId != null) {
>
> MovementWorkflow originalWorkflow =
>
> Workflow.newExternalWorkflowStub(
>
> MovementWorkflow.class, originalWorkflowId);
>
> try {
>
> originalWorkflow.receberFlagCancelamento();
>
> } catch (Exception e) {
>
> // Workflow original já terminou --- ok, tratamos pelo status no banco
>
> log.warn(\"Workflow original {} não encontrado (já finalizado?)\",
>
> originalWorkflowId);
>
> }
>
> }
>
> // ── PASSO 5: Verificar cadeia (escopo limitado nesta sprint) ──
>
> // TODO próxima sprint: implementar CA40-CA42
>
> // Por ora: apenas remove da cadeia se faz parte
>
> if (movimentoOriginal.getCodCadeia() != null) {
>
> flagCancelamento.removerDaCadeia(movimentoOriginal);
>
> }
>
> // ── PASSO 6: Fluxo por modalidade ──
>
> boolean isComFinanceiro = isComFinanceiro(movimentoOriginal);
>
> boolean foiParaCarteira = foiParaCarteira(movimentoOriginal);
>
> if (isComFinanceiro) {
>
> executarCancelamentoComFinanceiro(cancelamentoDTO, movimentoOriginal);
>
> } else if (foiParaCarteira) {
>
> executarCancelamentoSemFinanceiroComCarteira(
>
> cancelamentoDTO, movimentoOriginal);
>
> } else {
>
> // Sem financeiro e sem carteira: apenas atualiza status
>
> atualizaStatus.atualizarOriginal(movimentoOriginal.getMovementOrderId(),
> \"C8\");
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"Finalizado\", null);
>
> }
>
> }

**Fluxo sem financeiro com carteira**

> private void executarCancelamentoSemFinanceiroComCarteira(
>
> MovementOrderDTO cancelamentoDTO, MovementOrderDTO movimentoOriginal)
> {
>
> String statusOriginal = movimentoOriginal.getStatus();
>
> // CA44: original está Em execução --- aguardar finalização da
> carteira
>
> if (\"Em execução\".equals(statusOriginal)) {
>
> // O movimento original vai tratar a reversão ao receber a flag (CA48)
>
> // Aguardamos o retorno via Signal do unlock
>
> Workflow.await(Duration.ofMinutes(10), () -\> unlockSucesso != null);
>
> if (unlockSucesso == null) {
>
> log.error(\"Timeout aguardando retorno da carteira do original\");
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO,
>
> \"Finalizado\", buildTimeoutError());
>
> return;
>
> }
>
> }
>
> // CA45/CA46: original está Finalizado --- reversão direta
>
> ActivityReturnDTO\<Void\> reversaoResult =
>
> reversaoCarteira.reverter(movimentoOriginal);
>
> if (isError(reversaoResult)) {
>
> // CA49: erro na movimentação de carteira
>
> // Original fica C8 mesmo assim (erro funcional não é exibido)
>
> atualizaStatus.atualizarOriginal(
>
> movimentoOriginal.getMovementOrderId(), \"C8\");
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"Finalizado\",
>
> reversaoResult.getMessageError());
>
> return;
>
> }
>
> // CA47: sem saldo --- status especial
>
> if (reversaoResult.isSemSaldo()) {
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO,
>
> \"Rejeitado: sem saldo\", null);
>
> flagCancelamento.removerFlag(movimentoOriginal.getMovementOrderId());
>
> return;
>
> }
>
> // Sucesso
>
> atualizaStatus.atualizarOriginal(movimentoOriginal.getMovementOrderId(),
> \"C8\");
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"Finalizado\", null);
>
> }

**Fluxo com financeiro (modalidade bruta)**

> private void executarCancelamentoComFinanceiro(
>
> MovementOrderDTO cancelamentoDTO, MovementOrderDTO movimentoOriginal)
> {
>
> // CA36: verificar se já foi liquidado
>
> if (isLiquidado(movimentoOriginal)) {
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"E1\",
>
> buildError(\"Não é possível cancelar um movimento já liquidado\"));
>
> return;
>
> }
>
> // CA37.1: Pendente liquidação --- solicitar cancelamento da
> liquidação
>
> if (\"Pendente liquidação\".equals(movimentoOriginal.getStatus())) {
>
> log.info(\"Solicitando cancelamento da liquidação\");
>
> // Envia mensagem para o tópico de liquidação com Settlement type C
>
> message.sendMessage(buildLiquidacaoCancelamentoRequest(
>
> movimentoOriginal, cancelamentoDTO));
>
> // Aguarda retorno da liquidação via Signal (CA37.2)
>
> Workflow.await(Duration.ofMinutes(30), () -\> statusLiquidacao !=
> null);
>
> if (statusLiquidacao == null) {
>
> log.error(\"Timeout aguardando retorno da liquidação\");
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"E1\",
>
> buildTimeoutError());
>
> return;
>
> }
>
> if (\"Liquidada\".equals(statusLiquidacao)) {
>
> // CA37.2.1: liquidação já ocorreu --- não é possível cancelar
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"E1\",
>
> buildError(\"Não é possível cancelar um movimento já liquidado\"));
>
> return;
>
> }
>
> if (\"Cancelada\".equals(statusLiquidacao)) {
>
> // CA37.2.2: liquidação cancelada --- solicitar unlock
>
> message.sendMessage(buildUnlockRequest(movimentoOriginal));
>
> // Aguarda Signal de retorno do unlock (CA37.3)
>
> Workflow.await(Duration.ofMinutes(10), () -\> unlockSucesso != null);
>
> if (unlockSucesso == null \|\| !unlockSucesso) {
>
> // CA38: erro no lock/unlock
>
> atualizaStatus.atualizarOriginal(
>
> movimentoOriginal.getMovementOrderId(), \"Cancelado\");
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO,
>
> \"Finalizado\", null);
>
> return;
>
> }
>
> }
>
> }
>
> // CA37.3 / CA39: unlock realizado ou status já era outro
>
> atualizaStatus.atualizarOriginal(
>
> movimentoOriginal.getMovementOrderId(), \"C8\");
>
> atualizaStatus.atualizarCancelamento(cancelamentoDTO.getMovementOrderId(),
> cancelamentoDTO, \"Finalizado\", null);
>
> }

**Handlers dos Signals**

> // Handlers dos \@SignalMethod
>
> \@Override
>
> public void confirmarCancelamentoContraparte(MovementOrderDTO dto) {
>
> log.info(\"Signal: contraparte confirmou o cancelamento\");
>
> this.contraparteDTO = dto;
>
> this.contraparteConfirmou = true;
>
> }
>
> \@Override
>
> public void receberRetornoLiquidacao(String statusLiquidacao) {
>
> log.info(\"Signal: retorno liquidação = {}\", statusLiquidacao);
>
> this.statusLiquidacao = statusLiquidacao;
>
> }
>
> \@Override
>
> public void receberRetornoUnlock(boolean sucesso) {
>
> log.info(\"Signal: retorno unlock = {}\", sucesso);
>
> this.unlockSucesso = sucesso;
>
> }
>
> **4.3 Activities --- Novos arquivos**

**RecuperaMovimentoOriginalActivity**

Localização:
workflows/cancellation/activities/RecuperaMovimentoOriginalActivity.java

> Campo confirmado em MovementEntity: movementControlCode
> (COD_CONTROLE_MOVIMENTO, length=20)
>
> Já existe na entity --- sem migration necessária para este campo.
>
> Adicionar query no MovementRepository:
> findByMovementControlCode(String code)
>
> \@ActivityInterface
>
> public interface RecuperaMovimentoOriginalActivity {
>
> ActivityReturnDTO\<MovementOrderDTO\> recuperar(String
> movementControlCode);
>
> }
>
> // Adicionar ao MovementRepository:
>
> Optional\<MovementEntity\> findByMovementControlCode(String
> movementControlCode);
>
> // Impl:
>
> \@Component
>
> public class RecuperaMovimentoOriginalActivityImpl
>
> implements RecuperaMovimentoOriginalActivity {
>
> \@Autowired private MovementRepository movementRepository;
>
> \@Autowired private OrderEntryRepository orderEntryRepository;
>
> \@Override
>
> public ActivityReturnDTO\<MovementOrderDTO\> recuperar(String
> movementControlCode) {
>
> Optional\<MovementEntity\> opt =
>
> movementRepository.findByMovementControlCode(movementControlCode);
>
> if (opt.isEmpty()) {
>
> return ActivityDTOHelper.createError(
>
> \"Movimento original não encontrado para codControle: \"
>
> \+ movementControlCode); // CA20
>
> }
>
> MovementEntity movement = opt.get();
>
> // originLaunch = OrderEntryEntity (@ManyToOne, ID_LANCAMENTO_ORIGEM)
>
> // É o lançamento origem do movimento --- fonte dos dados para o DTO
>
> OrderEntryEntity originEntry = movement.getOriginLaunch();
>
> return ActivityDTOHelper.createSuccess(toMovementOrderDTO(movement,
> originEntry));
>
> }
>
> private MovementOrderDTO toMovementOrderDTO(
>
> MovementEntity movement, OrderEntryEntity entry) {
>
> MovementOrderDTO dto = new MovementOrderDTO();
>
> dto.setMovementOrderId(entry.getOrderEntryId());
>
> dto.setOperationTypeCode(String.valueOf(entry.getMovementTypeCode()));
>
> dto.setSelfNumber(entry.getMovementCode());
>
> dto.setOperationDate(entry.getOrderEntryId()); // ajustar conforme
> necessário
>
> // movementControlCode = codControleMovimento do movimento original
>
> dto.setCodControleMovimento(movement.getMovementControlCode());
>
> // \... mapear demais campos conforme necessidade do fluxo \...
>
> return dto;
>
> }
>
> }

**ValidaCancelamentoActivity**

Localização:
workflows/cancellation/activities/ValidaCancelamentoActivity.java

> \@ActivityInterface
>
> public interface ValidaCancelamentoActivity {
>
> ActivityReturnDTO\<Void\> validar(
>
> MovementOrderDTO cancelamento,
>
> MovementOrderDTO original);
>
> ActivityReturnDTO\<Void\> revalidarAposContraparte(
>
> MovementOrderDTO cancelamento,
>
> MovementOrderDTO original);
>
> }
>
> // Impl --- principais validações:
>
> \@Override
>
> public ActivityReturnDTO\<Void\> validar(
>
> MovementOrderDTO cancelamento, MovementOrderDTO original) {
>
> // CA21: mesmo dia
>
> if (!LocalDate.now(ZoneId.of(\"America/Sao_Paulo\"))
>
> .equals(original.getMovementDate())) {
>
> return ActivityDTOHelper.createError(
>
> \"Só é possível cancelar um movimento feito na data de hoje\");
>
> }
>
> // CA22/CA36: status cancelável
>
> String status = original.getStatus();
>
> if (\"Em execução\".equals(status) \|\| \"Finalizado\".equals(status))
> {
>
> if (isModalidadeBruta(original)) {
>
> return ActivityDTOHelper.createError(
>
> \"Não é possível cancelar um movimento já liquidado\");
>
> }
>
> }
>
> // CA51 (doc): código do cancelamento deve ser 01XX
>
> String codCancelamento = cancelamento.getOperationTypeCode();
>
> if (codCancelamento == null \|\| !codCancelamento.startsWith(\"01\"))
> {
>
> return ActivityDTOHelper.createError(\"Código do movimento é
> inválido\");
>
> }
>
> // CA19: validar grade
>
> // (reutilizar GradeValidationActivity existente)
>
> // \...
>
> return ActivityDTOHelper.createSuccess(null);
>
> }

**FlagCancelamentoActivity**

Localização:
workflows/cancellation/activities/FlagCancelamentoActivity.java

> \@ActivityInterface
>
> public interface FlagCancelamentoActivity {
>
> // Seta flag no movimento original e vincula os dois movimentos
> (CA50/CA34)
>
> void setarFlag(String movimentoOriginalId, String
> movimentoCancelamentoId);
>
> // Remove flag (quando cancelamento falha --- CA47)
>
> void removerFlag(String movimentoOriginalId);
>
> // Remove da cadeia se aplicável
>
> void removerDaCadeia(MovementOrderDTO movimentoOriginal);
>
> }
>
> // Impl:
>
> \@Override
>
> public void setarFlag(String movOriginalControlCode, String
> movCancelamentoId) {
>
> movementRepository.findByMovementControlCode(movOriginalId)
>
> .ifPresent(entity -\> {
>
> entity.setCancelamentoSolicitado(true);
>
> movementRepository.save(entity);
>
> log.info(\"\[FlagCancelamento\] flag setada para
> movementControlCode={}\", movOriginalId);
>
> });
>
> }

**ReversaoCarteiraActivity**

Localização:
workflows/cancellation/activities/ReversaoCarteiraActivity.java

> \@ActivityInterface
>
> public interface ReversaoCarteiraActivity {
>
> ActivityReturnDTO\<ReversaoCarteiraResult\> reverter(MovementOrderDTO
> original);
>
> }
>
> // Impl --- lógica inversa do WorkflowCarteira existente:
>
> \@Override
>
> public ActivityReturnDTO\<ReversaoCarteiraResult\> reverter(
>
> MovementOrderDTO original) {
>
> // Monta payload de reversão baseado no tipo de operação
>
> // CA46: compra e venda --- devolve quantidade ao vendedor
>
> // CA44/CA45: compra e venda sem modalidade
>
> ReversaoCarteiraRequest request = buildReversaoRequest(original);
>
> // Envia para o mesmo tópico de carteira, mas com operação inversa
>
> String topicMessage = objectMapper.writeValueAsString(request);
>
> messageAdapter.sendToCarteira(topicMessage);
>
> // Aguarda retorno do serviço externo de carteira
>
> // (Este método é uma Activity --- o await fica no Workflow)
>
> // Retorna o resultado para o CancelamentoWorkflowImpl decidir
>
> CarteiraRetorno retorno =
> carteiraClient.aguardarRetorno(request.getRequestId());
>
> if (!retorno.isSucesso()) {
>
> if (retorno.isSemSaldo()) {
>
> return ActivityDTOHelper.createSemSaldo();
>
> }
>
> return ActivityDTOHelper.createError(retorno.getMensagemErro());
>
> }
>
> return ActivityDTOHelper.createSuccess(
>
> new ReversaoCarteiraResult(true));
>
> }

**AtualizaStatusCancelamentoActivity**

Localização:
workflows/cancellation/activities/AtualizaStatusCancelamentoActivity.java

> Descoberta importante --- OrderEntryEntity:
>
> • O status é armazenado em launchStatusCode (COD_STATUS_LANCAMENTO,
> String, length=10)
>
> • ✅ Confirmado via OrderEntryService.saveOrderEntry() (linha 126): o
> padrão real é só setLaunchStatusCode(String) + save() --- sem FK
> orderEntryStatusEntity.
>
> • UpdateEntityAdapterImpl.getUpdatedOrderEntryEntity() não toca em
> launchStatusCode --- apenas atualiza participantPartyName e
> participantCounterpartyName. A impl da
> AtualizaStatusCancelamentoActivity deve usar o mesmo padrão do
> saveOrderEntry: setLaunchStatusCode(novoStatus) + save(), sem nenhum
> repository de status.
>
> \@ActivityInterface
>
> public interface AtualizaStatusCancelamentoActivity {
>
> // Atualiza launchStatusCode do lançamento original (ex: C8, C9)
>
> void atualizarOriginal(String orderEntryId, String novoStatus);
>
> // Atualiza launchStatusCode do cancelamento + notifica tópico de
> saída
>
> void atualizarCancelamento(
>
> String orderEntryId,
>
> MovementOrderDTO cancelamentoDTO,
>
> String novoStatus,
>
> MessageError erro);
>
> }
>
> // Impl:
>
> \@Override
>
> public void atualizarOriginal(String orderEntryId, String novoStatus)
> {
>
> orderEntryRepository.findByOrderEntryId(orderEntryId).ifPresent(entity
> -\> {
>
> // launchStatusCode = COD_STATUS_LANCAMENTO (String, length=10)
>
> // Padrão confirmado: só setLaunchStatusCode(String) + save() --- sem
> FK de status
>
> entity.setLaunchStatusCode(novoStatus);
>
> orderEntryRepository.save(entity);
>
> log.info(\"\[Cancelamento\] launchStatusCode={} em orderEntryId={}\",
>
> novoStatus, orderEntryId);
>
> });
>
> }
>
> \@Override
>
> public void atualizarCancelamento(
>
> String orderEntryId,
>
> MovementOrderDTO cancelamentoDTO,
>
> String novoStatus,
>
> MessageError erro) {
>
> // 1. Mesmo padrão de atualizarOriginal
>
> atualizarOriginal(orderEntryId, novoStatus);
>
> // 2. Notifica tópico de saída --- padrão do
> OrderEntryWorkflowAdapterImpl
>
> String notificacao = createStatusMap(novoStatus, orderEntryId, erro);
>
> messageAdapter.sendMessage(notificacao);
>
> }
>
> **5. Hot Config --- Configuração Parametrizada**

Todos os parâmetros do cancelamento devem vir do hot config, sem
necessidade de redeploy. Seguem os campos a configurar:

> Padrão já existente: getGeneralWorkflowConfiguration() busca via
> GetWorkflowConfigAdapter.
>
> O CancelamentoWorkflow deve usar a mesma infra, apenas com campos
> adicionais.

**Campos de configuração**

> // Exemplo de configuração esperada no hot config (formato JSON):
>
> {
>
> \"cancelamentoEnabled\": true,
>
> \"cancelamentoWorkflowQueue\": \"cancellation-workflow-queue\",
>
> \"cancelamentoValidations\": \[
>
> \"VALIDACAO_MOVIMENTO_ORIGINAL_EXISTE\",
>
> \"VALIDACAO_DATA_OPERACAO_D0\",
>
> \"VALIDACAO_STATUS_CANCELAVEL\",
>
> \"VALIDACAO_CODIGO_CANCELAMENTO\",
>
> \"VALIDACAO_GRADE_CANCELAMENTO\"
>
> \],
>
> \"cancelamentoDuploComandoTimeoutMinutes\": 1440,
>
> \"cancelamentoLiquidacaoTimeoutMinutes\": 30,
>
> \"cancelamentoCarteiraTimeoutMinutes\": 10
>
> }

**CancelamentoWorkflowConfiguration --- novo objeto**

> // Arquivo: domain/CancelamentoWorkflowConfiguration.java
>
> \@Data
>
> public class CancelamentoWorkflowConfiguration {
>
> private Boolean enabled;
>
> private String workflowQueue;
>
> private List\<LogicalValidationsEnum\> validations;
>
> private Integer duploComandoTimeoutMinutes;
>
> private Integer liquidacaoTimeoutMinutes;
>
> private Integer carteiraTimeoutMinutes;
>
> public boolean isEnabled() {
>
> return enabled != null && enabled;
>
> }
>
> public Duration getDuploComandoTimeout() {
>
> int mins = duploComandoTimeoutMinutes != null
>
> ? duploComandoTimeoutMinutes : 1440;
>
> return Duration.ofMinutes(mins);
>
> }
>
> }
>
> **6. Pontos de Atenção e Riscos**

**🔴 Signal para workflow em execução**

> O Signal via Workflow.newExternalWorkflowStub() só funciona se o
> MovementWorkflow
>
> estiver em execução no Temporal. Se o movimento original já finalizou
> (status Finalizado),
>
> o Signal vai falhar silenciosamente. Por isso o CancelamentoWorkflow
> deve sempre
>
> verificar o status no banco primeiro --- o Signal é apenas um
> otimizador para
>
> movimentos em andamento.

**🔴 workflowId deve ser persistido**

> A migration de banco é obrigatória antes do deploy. Sem o campo
> temporal_workflow_id
>
> na tabela de movimentos, o Signal não pode ser enviado ao workflow
> original.
>
> Garantir que o campo seja populado no momento do SaveOrderEntry.

**🟡 PARENT_CLOSE_POLICY para CancelamentoWorkflow**

> O CancelamentoWorkflow é disparado com PARENT_CLOSE_POLICY_ABANDON
> (não TERMINATE).
>
> Isso garante que o cancelamento continue mesmo se o OrderEntryWorkflow
> pai terminar.
>
> Diferente do MovementWorkflow atual que usa TERMINATE.

**🟡 Casamento duplo comando --- timing**

> No duplo comando, o diagrama diz: \'quando chegar o movimento de
> configuração pela
>
> contraparte, pare o casamento\'. O CasamentoWorkflow atual não tem
> suporte a isso.
>
> Para esta sprint: o Workflow.await() no CancelamentoWorkflow aguarda o
> Signal da
>
> contraparte, e a contraparte faz um novo lançamento que entra pelo
> fluxo normal.
>
> O casamento dos dois lançamentos de cancelamento será pelo
> codControleMovimento.

**🟢 Validações lógicas parametrizáveis**

> As validações do cancelamento seguem o mesmo padrão de
> LogicalValidationsEnum já
>
> existente. Isso significa que cada validação pode ser
> habilitada/desabilitada
>
> individualmente via hot config, sem redeploy.

**🟢 Reutilização de infrastructure existente**

> UpdateEntityAdapter, MessageAdapter e GradeValidationActivity são
> reutilizados
>
> sem alteração. O CancelamentoWorkflow só cria novas Activities para
> lógica
>
> específica do cancelamento.
>
> **7. Critérios de Aceite --- Cobertura**

  ------------------------------------------------------------------------------------------------
  **CA**                  **Descrição**           **Onde implementado**
  ----------------------- ----------------------- ------------------------------------------------
  CA13                    Obrigatoriedade dos     ValidaCancelamentoActivity.validar()
                          campos                  

  CA19                    Validar grade           ValidaCancelamentoActivity (reutiliza
                                                  GradeValidationActivity)

  CA20                    Movimento original não  RecuperaMovimentoOriginalActivity
                          encontrado              

  CA21                    Data operação original  ValidaCancelamentoActivity.validar()
                          \<\> D0                 

  CA22                    Validação no lançamento ValidaCancelamentoActivity.validar()

  CA23                    Confirmação do          CancelamentoWorkflowImpl ---
                          lançamento duplo        Workflow.await(contraparteConfirmou)

  CA24                    Cancelamento sem        CancelamentoWorkflowImpl --- revalidação após
                          confirmação             contraparte

  CA26                    Pendente lançamento     AtualizaStatusCancelamentoActivity ---
                          contraparte             resolveStatusPendente()

  CA27                    Não confirmação da      Signal não chega --- workflow aguarda
                          contraparte             indefinidamente

  CA28                    Confirmação da          Signal confirmarCancelamentoContraparte()
                          contraparte             

  CA34/CA29               Flag cancelamento no    FlagCancelamentoActivity.setarFlag()
                          original                

  CA31/CA35               Finalização do          AtualizaStatusCancelamentoActivity --- C8 +
                          cancelamento            Finalizado

  CA36                    Validação antes do      ValidaCancelamentoActivity --- isLiquidado()
                          cancelamento bruta      

  CA37.1                  Status Pendente         executarCancelamentoComFinanceiro()
                          liquidação              

  CA37.2                  Retorno da liquidação   Signal receberRetornoLiquidacao()

  CA37.3                  Lock/unlock da          Signal receberRetornoUnlock()
                          liquidação              

  CA43-CA45               Status movimento        executarCancelamentoSemFinanceiroComCarteira()
                          original sem financeiro 

  CA46                    Status finalizado       ReversaoCarteiraActivity
                          compra/venda            

  CA47                    Sem saldo               ReversaoCarteiraActivity --- isSemSaldo()

  CA48                    Status Em execução      Signal MovementWorkflow + Workflow.await()
                          original                

  CA50                    Flag cancelamento no    FlagCancelamentoActivity + Signal
                          original                receberFlagCancelamento()

  CA51 (code)             Código do movimento     ValidaCancelamentoActivity
                          01XX                    

  CA32/CA25               Sem confirmação / sem   isCancelamento() --- pula
                          comitente               IdentificacaoComitentes
  ------------------------------------------------------------------------------------------------

> **8. Pendências --- Questionar ao Time de Negócio**

-   CA37.1: Qual o timeout máximo para aguardar retorno da liquidação
    > antes de considerar erro?

-   CA27: Qual o timeout para a contraparte confirmar o duplo comando?
    > Existe prazo de expiração?

-   Cancelamento com status \'Pendente liquidação\': documento word
    > prometido pela Ana ainda não entregue --- detalhar comportamento.

-   Cancelamento de TSF: Ana confirmou que existe mas com baixa
    > frequência. Incluir nesta sprint ou próxima?

-   Flag de cancelamento na operação original: a solução de coluna nova
    > foi validada pelo time de arquitetura?

-   CA51/CA52: O \'aviso\' entre domínios após lock de IF é via Signal
    > Temporal ou evento Kafka? Definir o canal.
