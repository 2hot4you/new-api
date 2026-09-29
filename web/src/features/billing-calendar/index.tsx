import { useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  CalendarDays,
  Download,
  ExternalLink,
  RefreshCw,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import { Calendar, CalendarDayButton } from '@/components/ui/calendar'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Combobox } from '@/components/ui/combobox'
import { cn } from '@/lib/utils'

import { getBillingCalendar } from './api'
import {
  billingCalendarDay,
  buildBillingCalendarIcs,
  buildBillingDaySegments,
  type BillingCalendarClassification,
  type BillingCalendarMetadata,
} from './calendar'

const classificationLabels: Record<BillingCalendarClassification, string> = {
  workday: 'Workday',
  weekend: 'Weekend',
  holiday: 'Statutory holiday',
  makeup_workday: 'Makeup workday',
  unknown: 'Unavailable',
}

const classificationClasses: Record<BillingCalendarClassification, string> = {
  workday: 'border-sky-500/40 bg-sky-500/10 text-sky-700 dark:text-sky-300',
  weekend:
    'border-slate-500/40 bg-slate-500/10 text-slate-700 dark:text-slate-300',
  holiday: 'border-red-500/40 bg-red-500/10 text-red-700 dark:text-red-300',
  makeup_workday:
    'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
  unknown: 'border-zinc-500/40 bg-zinc-500/10 text-zinc-700 dark:text-zinc-300',
}

function dateKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(
    date.getDate()
  ).padStart(2, '0')}`
}

function initialDate(calendar: BillingCalendarMetadata): Date {
  const current = new Date()
  if (
    calendar.supported_years.includes(current.getFullYear()) &&
    dateKey(current) >= calendar.coverage_start &&
    dateKey(current) <= calendar.coverage_end
  ) {
    return current
  }
  const covered = new Date(`${calendar.coverage_start}T12:00:00`)
  return Number.isFinite(covered.getTime()) ? covered : current
}

function monthKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
}

function calendarMonthCovered(
  date: Date,
  calendar: BillingCalendarMetadata
): boolean {
  const year = date.getFullYear()
  if (!calendar.supported_years.includes(year)) return false
  const start = dateKey(new Date(year, date.getMonth(), 1, 12))
  const end = dateKey(new Date(year, date.getMonth() + 1, 0, 12))
  return end >= calendar.coverage_start && start <= calendar.coverage_end
}

function DayCell(props: {
  calendar: BillingCalendarMetadata
  dayProps: React.ComponentProps<typeof CalendarDayButton>
  label: (key: string) => string
}) {
  const key = dateKey(props.dayProps.day.date)
  const classification = billingCalendarDay(key, props.calendar).classification
  const text = props.label(classificationLabels[classification])
  return (
    <CalendarDayButton
      {...props.dayProps}
      aria-label={`${key} ${text}`}
      className={cn(
        'min-h-12 border text-[0.7rem]',
        classificationClasses[classification],
        props.dayProps.className
      )}
    >
      <span className='text-sm font-medium'>
        {props.dayProps.day.date.getDate()}
      </span>
      <span>{text}</span>
    </CalendarDayButton>
  )
}

function PriceSummary(props: { prices: Record<string, number | undefined> }) {
  const { t } = useTranslation()
  const labels: Record<string, string> = {
    p: 'Input',
    c: 'Output',
    cr: 'Cache read',
    cc: 'Cache write',
    cc1h: 'Cache create (1h) price',
    img: 'Image input price',
    img_cr: 'Image cache input price',
    img_o: 'Image output price',
    ai: 'Audio input price',
    ao: 'Audio output price',
  }
  return (
    <span className='text-muted-foreground text-xs'>
      {Object.entries(props.prices)
        .filter((entry): entry is [string, number] => entry[1] !== undefined)
        .map(
          ([name, value]) =>
            `${t(labels[name] ?? name)} $${value} / ${t('1M token')}`
        )
        .join(' · ')}
    </span>
  )
}

export function BillingCalendar() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['billing-calendar'],
    queryFn: getBillingCalendar,
    refetchOnWindowFocus: false,
  })
  const [selectedModel, setSelectedModel] = useState('')
  const [selectedDate, setSelectedDate] = useState<Date>()
  const [month, setMonth] = useState<Date>()
  const [evaluation, setEvaluation] = useState<ReturnType<
    typeof buildBillingDaySegments
  > | null>(null)
  const [evaluatedSelection, setEvaluatedSelection] = useState('')
  const [isEvaluating, setIsEvaluating] = useState(false)

  useEffect(() => {
    if (!query.data) return
    setSelectedModel(
      (current) => current || query.data.models[0]?.model_name || ''
    )
    setSelectedDate((current) => current ?? initialDate(query.data.calendar))
    setMonth((current) => current ?? initialDate(query.data.calendar))
  }, [query.data])

  const model = query.data?.models.find(
    (item) => item.model_name === selectedModel
  )
  const selectedKey = selectedDate ? dateKey(selectedDate) : ''
  const day =
    query.data && selectedKey
      ? billingCalendarDay(selectedKey, query.data.calendar)
      : null
  const evaluationSelection = model
    ? `${model.model_name}\u0000${model.billing_expr}\u0000${selectedKey}\u0000${query.data?.calendar.timezone ?? ''}`
    : ''
  useEffect(() => {
    if (!model || !selectedKey) {
      setEvaluation(null)
      setEvaluatedSelection('')
      setIsEvaluating(false)
      return
    }
    setEvaluation(null)
    setIsEvaluating(true)
    const timeout = window.setTimeout(() => {
      setEvaluation(
        buildBillingDaySegments(
          model.billing_expr,
          selectedKey,
          query.data?.calendar.timezone
        )
      )
      setEvaluatedSelection(evaluationSelection)
      setIsEvaluating(false)
    }, 0)
    return () => window.clearTimeout(timeout)
  }, [evaluationSelection, model, query.data?.calendar.timezone, selectedKey])
  const evaluationPending = Boolean(
    model &&
    selectedKey &&
    (isEvaluating || evaluatedSelection !== evaluationSelection)
  )
  const monthCovered = Boolean(
    query.data && month && calendarMonthCovered(month, query.data.calendar)
  )

  const exportCalendar = () => {
    if (!model || !month || !query.data) return
    const contents = buildBillingCalendarIcs({
      modelName: model.model_name,
      expression: model.billing_expr,
      month: monthKey(month),
      calendar: query.data.calendar,
    })
    const url = URL.createObjectURL(
      new Blob([contents], { type: 'text/calendar;charset=utf-8' })
    )
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${model.model_name.replaceAll(/[^A-Za-z0-9_-]+/g, '-')}-${monthKey(month)}.ics`
    anchor.click()
    URL.revokeObjectURL(url)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Billing calendar')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <a
          href='/models/metadata'
          className={buttonVariants({ variant: 'outline', size: 'sm' })}
        >
          {t('Open model pricing')}
          <ExternalLink className='size-4' />
        </a>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {query.isLoading && (
          <div
            role='status'
            aria-live='polite'
            className='text-muted-foreground flex items-center gap-2 py-10 text-sm'
          >
            <RefreshCw className='size-4 animate-spin' />
            {t('Loading billing calendar')}
          </div>
        )}
        {query.isError && (
          <Alert variant='destructive'>
            <AlertTriangle />
            <AlertTitle>{t('Unable to load billing calendar')}</AlertTitle>
            <AlertDescription>
              <Button
                variant='outline'
                size='sm'
                className='mt-2'
                onClick={() => void query.refetch()}
              >
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        )}
        {query.data && (
          <div className='space-y-4'>
            <Card>
              <CardHeader>
                <CardTitle>{t('Expression-priced model')}</CardTitle>
                <CardDescription>
                  {t(
                    'Search and select a model. Pricing remains read-only on this page.'
                  )}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Combobox
                  options={query.data.models.map((item) => ({
                    label: item.model_name,
                    value: item.model_name,
                  }))}
                  value={selectedModel}
                  onValueChange={(value) => setSelectedModel(value ?? '')}
                  aria-label={t('Billing model')}
                  placeholder={t('Select a billing model')}
                  emptyText={t('No expression-priced models')}
                  className='max-w-xl'
                />
              </CardContent>
            </Card>

            {query.data.models.length === 0 && (
              <Alert>
                <CalendarDays />
                <AlertTitle>{t('No expression-priced models')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'Configure expression pricing from the existing model pricing page.'
                  )}
                </AlertDescription>
              </Alert>
            )}

            {(!monthCovered || (day && !day.covered)) && (
              <Alert>
                <AlertTriangle />
                <AlertTitle>{t('Calendar year is not covered')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'Holiday data is unavailable for this year. The runtime safely treats unknown dates as non-holidays.'
                  )}
                </AlertDescription>
              </Alert>
            )}

            {model && selectedDate && month && (
              <div className='grid gap-4 xl:grid-cols-[minmax(0,1.1fr)_minmax(20rem,0.9fr)]'>
                <Card>
                  <CardHeader>
                    <CardTitle>{t('Month calendar')}</CardTitle>
                    <CardDescription>
                      {query.data.calendar.country} ·{' '}
                      {query.data.calendar.timezone}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className='space-y-3'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={exportCalendar}
                    >
                      <Download className='size-4' />
                      {t('Export Apple Calendar (.ics)')}
                    </Button>
                    <ul
                      className='flex flex-wrap gap-2'
                      aria-label={t('Calendar legend')}
                    >
                      {(
                        Object.keys(
                          classificationLabels
                        ) as BillingCalendarClassification[]
                      ).map((classification) => (
                        <li key={classification}>
                          <Badge
                            variant='outline'
                            className={classificationClasses[classification]}
                          >
                            {t(classificationLabels[classification])}
                          </Badge>
                        </li>
                      ))}
                    </ul>
                    <Calendar
                      mode='single'
                      selected={selectedDate}
                      month={month}
                      onMonthChange={setMonth}
                      onSelect={(date) => date && setSelectedDate(date)}
                      className='w-full [--cell-size:--spacing(12)]'
                      classNames={{
                        root: 'w-full',
                        month: 'w-full',
                        month_grid: 'w-full',
                      }}
                      components={{
                        DayButton: (dayProps) => (
                          <DayCell
                            calendar={query.data.calendar}
                            dayProps={dayProps}
                            label={t}
                          />
                        ),
                      }}
                    />
                    <p className='text-muted-foreground text-xs'>
                      {t('Source')}:{' '}
                      <a
                        href={query.data.calendar.source.url}
                        target='_blank'
                        rel='noreferrer'
                      >
                        {query.data.calendar.source.title}
                      </a>
                    </p>
                  </CardContent>
                </Card>

                <div className='space-y-4'>
                  <Card role='region' aria-label={t('Calendar classification')}>
                    <CardHeader>
                      <CardTitle>{t('Calendar classification')}</CardTitle>
                      <CardDescription>{selectedKey}</CardDescription>
                    </CardHeader>
                    <CardContent className='space-y-2'>
                      {day && (
                        <Badge
                          variant='outline'
                          className={classificationClasses[day.classification]}
                        >
                          {t(classificationLabels[day.classification])}
                        </Badge>
                      )}
                      <p className='text-muted-foreground text-sm'>
                        {t(
                          'This label describes the official calendar only. A makeup workday does not override the saved billing expression.'
                        )}
                      </p>
                    </CardContent>
                  </Card>

                  <Card
                    role='region'
                    aria-label={t('Expression result')}
                    aria-busy={evaluationPending}
                  >
                    <CardHeader>
                      <CardTitle>{t('Expression result')}</CardTitle>
                      <CardDescription>
                        {t('Evaluated independently at one-minute precision.')}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className='space-y-3'>
                      <span
                        role='status'
                        aria-label={t('Expression result')}
                        aria-live='polite'
                        className='sr-only'
                      >
                        {t(
                          evaluationPending
                            ? 'Loading billing calendar'
                            : 'Expression result'
                        )}
                      </span>
                      {evaluationPending && (
                        <div className='text-muted-foreground flex items-center gap-2 text-sm'>
                          <RefreshCw className='size-4 animate-spin' />
                          {t('Loading billing calendar')}
                        </div>
                      )}
                      {!evaluationPending && evaluation?.status === 'ready' && (
                        <p className='text-sm'>
                          <span className='font-medium'>
                            {t('Time-of-use pricing')}:
                          </span>{' '}
                          {t(evaluation.segments.length > 1 ? 'Yes' : 'No')}
                        </p>
                      )}
                      {!evaluationPending && evaluation?.status === 'ready' && (
                        <ol className='space-y-2'>
                          {evaluation.segments.map((segment) => (
                            <li
                              key={`${segment.start}-${segment.end}-${segment.tier}`}
                              className='flex flex-wrap items-center gap-x-2 gap-y-1 rounded-lg border p-2'
                            >
                              <span className='font-mono text-xs'>
                                {segment.start}–{segment.end}
                              </span>
                              <Badge variant='secondary'>{segment.tier}</Badge>
                              {segment.fixed_price === undefined ? (
                                <PriceSummary prices={segment.prices} />
                              ) : (
                                <span className='text-muted-foreground text-xs'>
                                  ${segment.fixed_price} / {t('request')}
                                </span>
                              )}
                            </li>
                          ))}
                        </ol>
                      )}
                      {!evaluationPending &&
                        evaluation &&
                        evaluation.status !== 'ready' && (
                          <Alert>
                            <AlertTriangle />
                            <AlertTitle>
                              {t('Expression cannot be visualized safely')}
                            </AlertTitle>
                            <AlertDescription>
                              {t(
                                'This expression uses conditions that cannot be represented as an exact daily price schedule. No prices were guessed.'
                              )}
                            </AlertDescription>
                          </Alert>
                        )}
                    </CardContent>
                  </Card>
                </div>
              </div>
            )}
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
