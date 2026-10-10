import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import axios from 'axios'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { toIntlLocale } from '@/i18n/languages'
import { handleServerError } from '@/lib/handle-server-error'
import { ROLE } from '@/lib/roles'
import { getServerErrorMessage } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import {
  applyCatalogSyncPlan,
  getCatalogSyncOperation,
  getCatalogSyncStatus,
  previewCatalogSync,
  previewCatalogSyncRestore,
  resolveCatalogSyncPlan,
} from './api'
import { CatalogChangelog } from './changelog'
import { ConfirmSyncDialog } from './confirm-sync-dialog'
import { CatalogHistory } from './history'
import type {
  CatalogBoundOperation,
  CatalogOperationBinding,
  CatalogSyncPlan,
  CatalogSyncResult,
} from './types'

const statusErrorLabels: Record<string, string> = {
  configuration_invalid: 'Catalog configuration is invalid.',
  external_origin_missing: 'Browser origin is not configured.',
  no_active_readers: 'No active source readers are configured.',
}

const bindingStorageKey = 'catalog-sync-original-operation'

// RequireSecurityProof returns these refusals before ApplyCatalogSyncPlan runs.
// Do not infer rollback from an arbitrary status, prefix, message or lookup 404.
const preWriteProofRefusals = new Set([
  'SECURITY_PROOF_REQUIRED',
  'SECURITY_PROOF_EXPIRED',
  'SECURITY_PROOF_SCOPE_MISMATCH',
  'SECURITY_METHOD_UNAVAILABLE',
  'SECURITY_PROOF_METHOD_MISMATCH',
  'SECURITY_PROOF_CONSUMED',
  'SECURITY_PROOF_CONTEXT_MISMATCH',
  'SECURITY_ACTION_FORBIDDEN',
  'SECURITY_PROOF_INVALID',
])

function readStoredBinding(userId: number): CatalogOperationBinding | null {
  try {
    const raw = window.sessionStorage.getItem(bindingStorageKey)
    if (!raw) return null
    const saved: unknown = JSON.parse(raw)
    if (!saved || typeof saved !== 'object') return null
    if (!('userId' in saved) || saved.userId !== userId) return null
    if (
      !('binding' in saved) ||
      !saved.binding ||
      typeof saved.binding !== 'object'
    ) {
      return null
    }
    const binding = saved.binding
    if (
      !('operationId' in binding) ||
      typeof binding.operationId !== 'string' ||
      !binding.operationId ||
      binding.operationId.length > 64
    ) {
      return null
    }
    if (
      !('planId' in binding) ||
      typeof binding.planId !== 'string' ||
      !/^[0-9a-f]{64}$/.test(binding.planId)
    ) {
      return null
    }
    if (
      !('digest' in binding) ||
      typeof binding.digest !== 'string' ||
      !/^[0-9a-f]{64}$/.test(binding.digest)
    ) {
      return null
    }
    return {
      operationId: binding.operationId,
      planId: binding.planId,
      digest: binding.digest,
    }
  } catch {
    return null
  }
}

function writeStoredBinding(
  userId: number,
  binding: CatalogOperationBinding | null
): void {
  try {
    if (binding) {
      window.sessionStorage.setItem(
        bindingStorageKey,
        JSON.stringify({ userId, binding })
      )
    } else {
      window.sessionStorage.removeItem(bindingStorageKey)
    }
  } catch {
    /* In-memory binding still protects this mounted view. */
  }
}

function committedReceipt(
  error: unknown,
  operationId: string
): CatalogSyncResult | null {
  if (!axios.isAxiosError<{ code?: string; data?: CatalogSyncResult }>(error)) {
    return null
  }
  const payload = error.response?.data
  if (payload?.code !== 'CATALOG_PUBLICATION_PENDING') {
    return null
  }
  return payload.data?.operation_id === operationId ? payload.data : null
}

export function CatalogSyncSection(): React.JSX.Element {
  const isRoot = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPER_ADMIN
  )
  if (!isRoot) return <span hidden />
  return <CatalogSyncRootSection />
}

function CatalogSyncRootSection(): React.JSX.Element {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const client = useQueryClient()
  const verification = useSecureVerification()
  const userId = useAuthStore((state) => state.auth.user?.id ?? 0)
  const [plan, setPlan] = useState<CatalogSyncPlan | null>(null)
  const [currentPlan, setCurrentPlan] = useState<CatalogSyncPlan | null>(null)
  const [finalPlan, setFinalPlan] = useState<CatalogSyncPlan | null>(null)
  const [selectedUnits, setSelectedUnits] = useState<string[]>([])
  const [confirmDeletes, setConfirmDeletes] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [binding, setBinding] = useState<CatalogOperationBinding | null>(() =>
    readStoredBinding(userId)
  )
  const [receipt, setReceipt] = useState<CatalogSyncResult | null>(null)
  const [feedback, setFeedback] = useState<string | null>(null)
  const [now, setNow] = useState(Date.now())

  useEffect(() => {
    if (!plan) return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [plan])

  const status = useQuery({
    queryKey: ['catalog-sync', 'status'],
    queryFn: getCatalogSyncStatus,
    retry: false,
    meta: { errorToast: false },
  })
  const ready = status.data?.role === 'target' && status.data.management_ready

  function acceptPreview(next: CatalogSyncPlan) {
    setPlan(next)
    setCurrentPlan(next)
    setFinalPlan(null)
    setSelectedUnits(next.resolution.overwrite_keys ?? [])
    setConfirmDeletes(next.resolution.confirm_deletes)
    setConfirmOpen(false)
    setBinding(null)
    setReceipt(null)
    writeStoredBinding(userId, null)
    setFeedback(null)
  }

  const preview = useMutation({
    mutationFn: previewCatalogSync,
    retry: false,
    onSuccess: acceptPreview,
    onError: (error) => setFeedback(getServerErrorMessage(error)),
  })
  const restore = useMutation({
    mutationFn: previewCatalogSyncRestore,
    retry: false,
    onSuccess: acceptPreview,
    onError: (error) => setFeedback(getServerErrorMessage(error)),
  })
  const resolve = useMutation({
    mutationFn: (choices: {
      source: CatalogSyncPlan
      units: string[]
      deletes: boolean
    }) =>
      resolveCatalogSyncPlan(
        choices.source.id,
        choices.source.digest,
        choices.units,
        choices.deletes
      ),
    retry: false,
    onSuccess: (result) => {
      setCurrentPlan(result)
      setFinalPlan(result)
      if (!result.executable) {
        setFeedback(
          t('Saved plan is blocked. Resolve conflicts or preview again.')
        )
      } else {
        setFeedback(null)
      }
    },
    onError: (error) => setFeedback(getServerErrorMessage(error)),
  })
  const apply = useMutation({
    mutationFn: (input: { binding: CatalogOperationBinding; proof: string }) =>
      applyCatalogSyncPlan(input.binding, input.proof),
    retry: false,
    onSuccess: (result) => {
      setReceipt(result)
      void client.invalidateQueries({ queryKey: ['catalog-sync', 'history'] })
    },
    onError: (error, input) => {
      const known = committedReceipt(error, input.binding.operationId)
      if (known) setReceipt(known)
      setFeedback(getServerErrorMessage(error))
      const response = axios.isAxiosError<{ success?: boolean; code?: string }>(
        error
      )
        ? error.response
        : undefined
      const code = response?.data?.code
      const proofRefused =
        response?.status === 403 &&
        response.data?.success === false &&
        preWriteProofRefusals.has(code ?? '')
      const invalidPlan =
        code === 'CATALOG_CONFLICT' || code === 'CATALOG_PLAN_BLOCKED'
      const savedReceipt = client.getQueryData<CatalogBoundOperation>([
        'catalog-sync',
        'receipt',
        input.binding.operationId,
        input.binding.planId,
        input.binding.digest,
      ])?.receipt
      if (
        !known &&
        !receipt &&
        !savedReceipt &&
        (proofRefused || invalidPlan)
      ) {
        setBinding(null)
        writeStoredBinding(userId, null)
        // Proof refusal does not invalidate the saved plan. A user may confirm
        // it again with a fresh operation/proof; normal expiry checks still apply.
        if (invalidPlan) {
          setPlan(null)
          setCurrentPlan(null)
          setFinalPlan(null)
        }
      }
      void client.invalidateQueries({ queryKey: ['catalog-sync', 'history'] })
    },
  })

  const bound = useQuery({
    queryKey: [
      'catalog-sync',
      'receipt',
      binding?.operationId,
      binding?.planId,
      binding?.digest,
    ],
    queryFn: () => {
      if (!binding) {
        throw new Error('Missing operation binding')
      }
      return getCatalogSyncOperation(binding.operationId, binding)
    },
    enabled: binding !== null && !apply.isPending,
    retry: false,
    refetchInterval: (query) =>
      query.state.data?.operation?.state === 'committed_pending_publish'
        ? 3000
        : false,
    meta: { errorToast: false },
  })
  const knownReceipt = receipt ?? bound.data?.receipt ?? null
  const unresolved = binding !== null && knownReceipt === null
  const publicationPending =
    binding !== null &&
    ((knownReceipt?.state === 'committed_pending_publish' &&
      bound.data?.operation?.state !== 'succeeded') ||
      bound.data?.operation === null ||
      (bound.data?.operation !== undefined &&
        bound.data.operation.state !== 'succeeded'))
  const mutationLocked = unresolved || publicationPending
  const busy =
    preview.isPending ||
    restore.isPending ||
    resolve.isPending ||
    apply.isPending ||
    verification.isActive
  const activePlan = finalPlan ?? currentPlan
  const expired = activePlan !== null && activePlan.expires_at * 1000 <= now
  const blocked =
    plan?.changes.some((change) => change.action === 'blocked') ?? false
  const hasCandidate =
    plan?.changes.some((change) =>
      ['create', 'update', 'delete', 'conflict', 'adopt'].includes(
        change.action
      )
    ) ?? false

  async function confirm() {
    if (!plan || !currentPlan || busy || expired || mutationLocked) return
    if (!finalPlan) {
      resolve.mutate({
        source: currentPlan,
        units: selectedUnits,
        deletes: confirmDeletes,
      })
      return
    }
    if (!finalPlan.executable) return
    setConfirmOpen(false)
    const pendingBinding: CatalogOperationBinding = {
      operationId: crypto.randomUUID(),
      planId: finalPlan.id,
      digest: finalPlan.digest,
    }
    let sent = false
    try {
      const context = {
        plan_digest: finalPlan.digest,
        target_id: finalPlan.target_id,
        operation_id: pendingBinding.operationId,
      }
      const proof =
        finalPlan.kind === 'restore'
          ? await verification.requestVerification({
              scope: 'catalog.sync.restore',
              context: { ...context, kind: 'restore' },
            })
          : await verification.requestVerification({
              scope: 'catalog.sync.apply',
              context: { ...context, kind: 'sync' },
            })
      if (!proof) return
      if (finalPlan.expires_at * 1000 <= Date.now()) {
        setFeedback(t('Check dev updates again to create a fresh preview.'))
        setNow(Date.now())
        return
      }
      setBinding(pendingBinding)
      writeStoredBinding(userId, pendingBinding)
      setReceipt(null)
      setFeedback(null)
      sent = true
      await apply.mutateAsync({
        binding: pendingBinding,
        proof: proof.proof_token,
      })
    } catch (error) {
      if (!sent) handleServerError(error)
    }
  }

  if (status.isPending) return <LoadingState />
  if (status.isError) {
    return (
      <ErrorState
        title={t('Catalog status is unavailable')}
        onRetry={() => void status.refetch()}
      />
    )
  }

  return (
    <section aria-label={t('Environment sync')} className='flex flex-col gap-6'>
      <header className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <h1 className='text-xl font-semibold'>{t('Environment sync')}</h1>
          <p className='text-muted-foreground'>
            {t('Role')}: {t(status.data.role)} · {t('Source identity')}:{' '}
            {status.data.source_id ?? '—'} · {t('Target site')}:{' '}
            {status.data.target_id ?? '—'}
          </p>
          {status.data.role === 'source' && (
            <p className='text-muted-foreground'>
              {t('Source ready')}: {t(status.data.source_ready ? 'Yes' : 'No')}
            </p>
          )}
          {status.data.role === 'target' && (
            <p className='text-muted-foreground'>
              {t('Target ready')}: {t(status.data.target_ready ? 'Yes' : 'No')}{' '}
              · {t('Management ready')}:{' '}
              {t(status.data.management_ready ? 'Yes' : 'No')}
            </p>
          )}
        </div>
        {ready && (
          <Button
            type='button'
            disabled={busy || mutationLocked}
            onClick={() => preview.mutate()}
          >
            {t('Check dev updates')}
          </Button>
        )}
      </header>
      {status.data.error && (
        <Alert>
          <AlertTitle>{t('Catalog status')}</AlertTitle>
          <AlertDescription>
            {t(
              statusErrorLabels[status.data.error] ??
                'Catalog status is unavailable'
            )}
          </AlertDescription>
        </Alert>
      )}
      {!ready && (
        <Alert>
          <AlertTitle>{t('Read-only status')}</AlertTitle>
          <AlertDescription>
            {t(
              'Catalog changes are available only on a ready target environment.'
            )}
          </AlertDescription>
        </Alert>
      )}
      {feedback && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Catalog operation notice')}</AlertTitle>
          <AlertDescription>{feedback}</AlertDescription>
        </Alert>
      )}
      {ready && plan && (
        <>
          <section className='flex flex-col gap-3'>
            <h2 className='text-lg font-medium'>
              {t(
                plan.kind === 'restore'
                  ? 'Restore preview'
                  : 'Dev update preview'
              )}
            </h2>
            <p className='text-muted-foreground text-sm'>
              {t('Target site')}: {plan.target_id} · {t('Source identity')}:{' '}
              {plan.source_id}
            </p>
            <p className='text-muted-foreground text-sm'>
              {t(
                plan.kind === 'restore'
                  ? 'Inverse snapshot time'
                  : 'Dev snapshot time'
              )}
              :{' '}
              {new Intl.DateTimeFormat(locale, {
                dateStyle: 'medium',
                timeStyle: 'short',
              }).format(new Date(plan.source_exported_at * 1000))}
            </p>
            <p className='text-muted-foreground font-mono text-xs break-all'>
              {t(
                plan.kind === 'restore'
                  ? 'Inverse snapshot digest'
                  : 'Dev snapshot digest'
              )}
              : {plan.source_digest}
            </p>
            {expired && (
              <Alert variant='destructive'>
                <AlertTitle>{t('Preview expired')}</AlertTitle>
                <AlertDescription>
                  {t('Check dev updates again to create a fresh preview.')}
                </AlertDescription>
              </Alert>
            )}
            <CatalogChangelog
              kind={plan.kind}
              changes={plan.changes}
              selectedUnits={selectedUnits}
              onSelectedUnitsChange={(units) => {
                setSelectedUnits(units)
                setFinalPlan(null)
              }}
              disabled={busy || expired}
            />
            <Button
              type='button'
              className='self-start'
              disabled={
                busy || expired || blocked || !hasCandidate || mutationLocked
              }
              onClick={() => setConfirmOpen(true)}
            >
              {t('Review and confirm')}
            </Button>
            {!hasCandidate && (
              <p>{t('No executable changes in this preview.')}</p>
            )}
            {blocked && (
              <p role='alert'>
                {t(
                  'Blocked changes must be resolved outside this sync before applying.'
                )}
              </p>
            )}
          </section>
          <ConfirmSyncDialog
            open={confirmOpen}
            onOpenChange={setConfirmOpen}
            plan={
              currentPlan ? { ...currentPlan, changes: plan.changes } : plan
            }
            finalPlan={finalPlan}
            confirmDeletes={confirmDeletes}
            onConfirmDeletesChange={(value) => {
              setConfirmDeletes(value)
              setFinalPlan(null)
            }}
            selectedCount={selectedUnits.length}
            loading={resolve.isPending || apply.isPending}
            onConfirm={() => void confirm()}
          />
        </>
      )}
      {binding && (
        <section
          aria-label={t('Operation result')}
          className='flex flex-col gap-2'
        >
          <h2 className='text-lg font-medium'>{t('Operation result')}</h2>
          <p className='font-mono text-xs break-all'>
            {t('Operation ID')}: {binding.operationId}
          </p>
          {apply.isPending && (
            <LoadingState
              inline
              message={t(
                'Submitting once; checking this operation for its result.'
              )}
            />
          )}
          {knownReceipt && (
            <p>
              {t('Immutable receipt')}: {t(knownReceipt.state)} ·{' '}
              {t('Revision')}: {knownReceipt.revision}
            </p>
          )}
          {bound.data?.operation && (
            <p>
              {t('Live publication state')}: {t(bound.data.operation.state)}
            </p>
          )}
          {knownReceipt && bound.data?.operation === null && (
            <p>
              {t(
                'Submission is confirmed; live publication state is unavailable.'
              )}
            </p>
          )}
          {unresolved && !apply.isPending && (
            <Alert>
              <AlertTitle>{t('Result unknown')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Keep this operation ID and check its original result. Do not submit the write again.'
                )}
              </AlertDescription>
            </Alert>
          )}
          {bound.isError && (
            <p>
              {t(
                'Receipt lookup is unavailable; the original operation remains unresolved.'
              )}
            </p>
          )}
          <Button
            type='button'
            variant='outline'
            className='self-start'
            disabled={apply.isPending || bound.isFetching}
            onClick={() => void bound.refetch()}
          >
            {t('Check original result')}
          </Button>
        </section>
      )}
      {ready && (
        <CatalogHistory
          onRestore={(id) => restore.mutate(id)}
          disabled={busy || mutationLocked}
        />
      )}
      <SecureVerificationDialog {...verification.dialogProps} />
    </section>
  )
}
