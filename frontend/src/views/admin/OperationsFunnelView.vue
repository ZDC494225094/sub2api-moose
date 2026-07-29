<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="shortcut in shortcuts"
            :key="shortcut.key"
            type="button"
            class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
            :class="activeShortcut === shortcut.key
              ? 'border-primary-600 bg-primary-600 text-white'
              : 'border-gray-200 text-gray-600 hover:bg-gray-100 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
            @click="applyShortcut(shortcut.key)"
          >
            {{ shortcut.label }}
          </button>
          <div class="flex items-center gap-1.5">
            <input v-model="startDate" type="date" class="input input-sm text-xs" @change="onCustomRange" />
            <span class="text-xs text-gray-400">-</span>
            <input v-model="endDate" type="date" class="input input-sm text-xs" @change="onCustomRange" />
          </div>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadFunnel">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div v-if="loading && !funnel" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="funnel">
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div v-for="metric in summaryMetrics" :key="metric.key" class="card p-4">
            <div class="flex items-center gap-3">
              <div :class="['rounded-lg p-2', metric.iconBg]">
                <Icon :name="metric.icon" size="md" :class="metric.iconColor" :stroke-width="2" />
              </div>
              <div class="min-w-0">
                <p class="truncate text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">{{ metric.value }}</p>
                <p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ metric.hint }}</p>
              </div>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
          <section class="card p-4">
            <div class="mb-4 flex items-center gap-3">
              <div class="rounded-lg bg-sky-100 p-2 dark:bg-sky-900/30">
                <Icon name="users" size="md" class="text-sky-600 dark:text-sky-400" :stroke-width="2" />
              </div>
              <div>
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.usersTitle') }}</h2>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.usersDescription') }}</p>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <button
                v-for="item in userSummaryItems"
                :key="item.key"
                type="button"
                class="rounded-lg bg-gray-50 p-3 text-left transition-colors disabled:cursor-default dark:bg-dark-700"
                :class="item.segment ? 'hover:bg-primary-50 dark:hover:bg-primary-950/20' : ''"
                :disabled="!item.segment"
                @click="item.segment && openUserDetails(item.segment, item.label)"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
                <div class="mt-1 flex items-center justify-between gap-2">
                  <span class="text-base font-semibold text-gray-900 dark:text-white">{{ item.value }}</span>
                  <Icon v-if="item.segment" name="chevronRight" size="xs" class="text-gray-400" />
                </div>
              </button>
            </div>
          </section>

          <section class="card p-4">
            <div class="mb-4 flex items-center gap-3">
              <div class="rounded-lg bg-teal-100 p-2 dark:bg-teal-900/30">
                <Icon name="database" size="md" class="text-teal-600 dark:text-teal-400" :stroke-width="2" />
              </div>
              <div>
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.creditsTitle') }}</h2>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.creditsDescription') }}</p>
              </div>
            </div>
            <div class="space-y-2">
              <button
                v-for="item in creditSummaryItems"
                :key="item.key"
                type="button"
                class="flex w-full items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-left transition-colors hover:bg-primary-50 dark:bg-dark-700 dark:hover:bg-primary-950/20"
                @click="openUserDetails(item.segment, item.label)"
              >
                <span class="block min-w-0 flex-1">
                  <span class="flex items-center justify-between gap-3">
                    <span class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</span>
                    <span class="flex items-center gap-1 text-sm font-semibold text-gray-900 dark:text-white">
                      {{ item.value }}
                      <Icon name="chevronRight" size="xs" class="text-gray-400" />
                    </span>
                  </span>
                  <span class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[11px] text-gray-400 dark:text-dark-400">
                    <span v-for="detail in item.details" :key="detail.label">{{ detail.label }} {{ detail.value }}</span>
                  </span>
                </span>
              </button>
            </div>
          </section>

          <section class="card p-4">
            <div class="mb-4 flex items-center gap-3">
              <div class="rounded-lg bg-violet-100 p-2 dark:bg-violet-900/30">
                <Icon name="gift" size="md" class="text-violet-600 dark:text-violet-400" :stroke-width="2" />
              </div>
              <div>
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.subscriptionsTitle') }}</h2>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.subscriptionsDescription') }}</p>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <button
                v-for="item in subscriptionSummaryItems"
                :key="item.key"
                type="button"
                class="rounded-lg bg-gray-50 p-3 text-left transition-colors hover:bg-primary-50 dark:bg-dark-700 dark:hover:bg-primary-950/20"
                @click="openUserDetails('subscription', item.label)"
              >
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
                <div class="mt-1 flex items-center justify-between gap-2">
                  <span class="text-base font-semibold text-gray-900 dark:text-white">{{ item.value }}</span>
                  <Icon name="chevronRight" size="xs" class="text-gray-400" />
                </div>
              </button>
            </div>
          </section>
        </div>

        <section v-if="breakdownGroups.length" class="card p-4">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.breakdownTitle') }}</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.breakdownDescription') }}</p>
            </div>
          </div>
          <div class="grid grid-cols-1 gap-4 lg:grid-cols-4">
            <div v-for="group in breakdownGroups" :key="group.key" class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
              <h3 class="mb-3 text-xs font-semibold uppercase text-gray-500 dark:text-gray-400">{{ group.label }}</h3>
              <div class="space-y-2">
                <button
                  v-for="item in group.items"
                  :key="item.key"
                  type="button"
                  class="flex w-full items-center justify-between gap-3 rounded-md px-2 py-1.5 text-left transition-colors"
                  :class="item.segment ? 'hover:bg-gray-50 dark:hover:bg-dark-700' : 'cursor-default'"
                  :disabled="!item.segment"
                  @click="item.segment && openUserDetails(item.segment, item.label)"
                >
                  <span class="min-w-0 truncate text-xs text-gray-600 dark:text-gray-300">{{ item.label }}</span>
                  <span class="shrink-0 text-xs font-semibold text-gray-900 dark:text-white">{{ item.value }}</span>
                </button>
              </div>
            </div>
          </div>
        </section>

        <div class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,0.85fr)]">
          <section class="card p-4">
            <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.funnelTitle') }}</h2>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.funnelDescription') }}</p>
              </div>
              <div class="text-right text-xs text-gray-500 dark:text-gray-400">
                <div>{{ t('admin.operations.range') }}: {{ funnel.start_date }} - {{ funnel.end_date }}</div>
                <div>{{ t('admin.operations.generatedAt') }}: {{ formatDateTime(funnel.generated_at) }}</div>
              </div>
            </div>

            <div class="space-y-3">
              <div v-for="step in localizedSteps" :key="step.key" class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div class="flex items-center gap-2">
                    <span :class="['flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold', step.badgeClass]">
                      {{ step.index + 1 }}
                    </span>
                    <div>
                      <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ step.label }}</div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t('admin.operations.conversionFromPrevious') }} {{ formatPercent(step.conversion_rate) }}
                      </div>
                    </div>
                  </div>
                  <div class="text-right">
                    <div class="text-lg font-bold text-gray-900 dark:text-white">{{ formatNumber(step.count) }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.operations.overallConversion') }} {{ formatPercent(step.overall_conversion_rate) }}
                    </div>
                  </div>
                </div>
                <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                  <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: step.barWidth }"></div>
                </div>
                <div v-if="step.index > 0" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.operations.dropoff') }}: {{ formatNumber(step.dropoff_from_previous) }}
                </div>
              </div>
            </div>
          </section>

          <div class="space-y-6">
            <section class="card p-4">
              <div class="mb-4 flex items-center justify-between gap-3">
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.revenueTitle') }}</h2>
                <RouterLink :to="ordersLink('COMPLETED', 'paid_at')" class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400">
                  {{ t('admin.operations.viewOrders') }}
                </RouterLink>
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div v-for="item in revenueItems" :key="item.key" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
                  <div class="mt-1 text-base font-semibold text-gray-900 dark:text-white">{{ item.value }}</div>
                </div>
              </div>
            </section>

            <section class="card p-4">
              <h2 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.signalsTitle') }}</h2>
              <div class="space-y-2">
                <RouterLink
                  v-for="signal in signalItems"
                  :key="signal.key"
                  :to="ordersLink(signal.status)"
                  class="flex items-center justify-between rounded-lg px-3 py-2 transition-colors hover:bg-gray-50 dark:hover:bg-dark-700"
                >
                  <div class="flex items-center gap-2">
                    <span :class="['inline-block h-2.5 w-2.5 rounded-full', signal.dotClass]"></span>
                    <span class="text-sm text-gray-700 dark:text-gray-300">{{ signal.label }}</span>
                  </div>
                  <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ formatNumber(signal.value) }}</span>
                </RouterLink>
              </div>
            </section>

            <section class="card p-4">
              <div class="mb-4 flex items-center gap-3">
                <div class="rounded-lg bg-rose-100 p-2 dark:bg-rose-900/30">
                  <Icon name="mail" size="md" class="text-rose-600 dark:text-rose-400" :stroke-width="2" />
                </div>
                <div>
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.marketing.title') }}</h2>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.marketing.description') }}</p>
                </div>
              </div>

              <div class="space-y-3">
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.subject') }}</label>
                  <input v-model="marketingForm.subject" type="text" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.subjectPlaceholder')" maxlength="120" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.body') }}</label>
                  <textarea v-model="marketingForm.body" class="input min-h-32 text-xs" rows="6" :placeholder="t('admin.operations.marketing.bodyPlaceholder')" maxlength="10000"></textarea>
                </div>
                <div class="grid grid-cols-2 gap-3">
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.audience') }}</label>
                    <select v-model="marketingForm.audience" class="input input-sm text-xs">
                      <option v-for="option in marketingAudienceOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                    </select>
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.status') }}</label>
                    <select v-model="marketingForm.status" class="input input-sm text-xs">
                      <option v-for="option in marketingStatusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                    </select>
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.bodyFormat') }}</label>
                    <select v-model="marketingForm.body_format" class="input input-sm text-xs">
                      <option v-for="option in marketingBodyFormatOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                    </select>
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.activeDays') }}</label>
                    <input v-model.number="marketingForm.active_days" type="number" min="1" max="365" class="input input-sm text-xs" />
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.minBalance') }}</label>
                    <input v-model="marketingForm.min_balance" type="number" min="0" step="0.01" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.balancePlaceholder')" />
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.maxBalance') }}</label>
                    <input v-model="marketingForm.max_balance" type="number" min="0" step="0.01" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.balancePlaceholder')" />
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.minTotalRecharged') }}</label>
                    <input v-model="marketingForm.min_total_recharged" type="number" min="0" step="0.01" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.balancePlaceholder')" />
                  </div>
                  <div>
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.maxTotalRecharged') }}</label>
                    <input v-model="marketingForm.max_total_recharged" type="number" min="0" step="0.01" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.balancePlaceholder')" />
                  </div>
                  <div class="col-span-2">
                    <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.limit') }}</label>
                    <input v-model.number="marketingForm.limit" type="number" min="1" max="100" class="input input-sm text-xs" />
                  </div>
                </div>

                <div class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <div class="text-xs font-medium text-gray-700 dark:text-gray-200">{{ t('admin.operations.marketing.selectedUsers') }}</div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t('admin.operations.marketing.selectedUsersHint', { count: selectedMarketingUserIds.length }) }}
                      </div>
                    </div>
                    <div class="flex items-center gap-2">
                      <button type="button" class="btn btn-secondary btn-sm" @click="openRecipientDialog">
                        <Icon name="filter" size="sm" />
                        {{ t('admin.operations.marketing.selectUsers') }}
                      </button>
                      <button v-if="selectedMarketingUserIds.length" type="button" class="btn btn-ghost btn-sm" @click="clearSelectedRecipients">
                        {{ t('admin.operations.marketing.clearSelected') }}
                      </button>
                    </div>
                  </div>
                </div>

                <div class="flex flex-wrap items-center gap-2">
                  <button type="button" class="btn btn-secondary flex-1" :disabled="marketingPreviewLoading || marketingSending" @click="previewMarketingEmail">
                    <Icon name="eye" size="sm" :class="marketingPreviewLoading ? 'animate-spin' : ''" />
                    {{ t('admin.operations.marketing.preview') }}
                  </button>
                  <button type="button" class="btn btn-primary flex-1" :disabled="marketingPreviewLoading || marketingSending" @click="sendMarketingEmail">
                    <Icon name="mail" size="sm" :class="marketingSending ? 'animate-spin' : ''" />
                    {{ t('admin.operations.marketing.send') }}
                  </button>
                </div>

                <div v-if="marketingResult" class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                  <div class="mb-2 grid grid-cols-3 gap-2 text-center">
                    <div>
                      <div class="text-base font-semibold text-gray-900 dark:text-white">{{ formatNumber(marketingResult.total_matched) }}</div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.marketing.matched') }}</div>
                    </div>
                    <div>
                      <div class="text-base font-semibold text-gray-900 dark:text-white">{{ formatNumber(marketingResult.targeted) }}</div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.marketing.targeted') }}</div>
                    </div>
                    <div>
                      <div class="text-base font-semibold text-gray-900 dark:text-white">{{ formatNumber(marketingResult.sent) }}</div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.marketing.sent') }}</div>
                    </div>
                  </div>
                  <div class="mb-3 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ t('admin.operations.marketing.failed') }}: {{ formatNumber(marketingResult.failed) }}</span>
                    <span>{{ t('admin.operations.marketing.skippedInvalid') }}: {{ formatNumber(marketingResult.skipped_invalid) }}</span>
                    <span>{{ t('admin.operations.marketing.limit') }}: {{ formatNumber(marketingResult.limit) }}</span>
                  </div>
                  <div v-if="marketingResult.sample?.length" class="space-y-1">
                    <div class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.operations.marketing.sample') }}</div>
                    <div v-for="recipient in marketingResult.sample" :key="recipient.user_id" class="truncate rounded-md bg-gray-50 px-2 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                      #{{ recipient.user_id }} {{ recipient.email }}
                    </div>
                  </div>
                  <div v-if="marketingResult.errors?.length" class="mt-3 space-y-1">
                    <div class="text-xs font-medium text-red-600 dark:text-red-400">{{ t('admin.operations.marketing.errors') }}</div>
                    <div v-for="error in marketingResult.errors" :key="error" class="break-all text-xs text-red-600 dark:text-red-400">{{ error }}</div>
                  </div>
                </div>
              </div>
            </section>

            <section class="card p-4">
              <div class="mb-4 flex items-center justify-between gap-3">
                <div>
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.marketing.recordsTitle') }}</h2>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.marketing.recordsDescription') }}</p>
                </div>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="marketingRecordsLoading" @click="loadMarketingRecords">
                  <Icon name="refresh" size="sm" :class="marketingRecordsLoading ? 'animate-spin' : ''" />
                </button>
              </div>
              <div v-if="marketingRecordsLoading && marketingRecords.length === 0" class="py-6">
                <LoadingSpinner />
              </div>
              <div v-else-if="marketingRecords.length" class="space-y-2">
                <div v-for="record in marketingRecords" :key="record.id" class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <div class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ record.subject }}</div>
                      <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(record.created_at) }}</div>
                    </div>
                    <div class="shrink-0 text-right text-xs">
                      <div class="font-semibold text-emerald-600 dark:text-emerald-400">{{ t('admin.operations.marketing.sent') }} {{ formatNumber(record.sent) }}</div>
                      <div class="text-red-600 dark:text-red-400">{{ t('admin.operations.marketing.failed') }} {{ formatNumber(record.failed) }}</div>
                    </div>
                  </div>
                  <div class="mt-2 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ t('admin.operations.marketing.targeted') }} {{ formatNumber(record.targeted) }}</span>
                    <span>{{ t('admin.operations.marketing.selected') }} {{ formatNumber(record.selected_user_count) }}</span>
                    <span>{{ marketingAudienceLabel(record.audience) }}</span>
                  </div>
                  <p v-if="record.body_preview" class="mt-2 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">{{ record.body_preview }}</p>
                </div>
              </div>
              <div v-else class="rounded-lg border border-dashed border-gray-200 py-6 text-center text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
                {{ t('admin.operations.marketing.noRecords') }}
              </div>
            </section>
          </div>
        </div>
      </template>

      <div v-else class="card flex items-center justify-center py-12 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.operations.noData') }}
      </div>

      <BaseDialog
        :show="userDetailsDialog.show"
        :title="userDetailsDialog.title"
        width="extra-wide"
        @close="userDetailsDialog.show = false"
      >
        <div class="space-y-4">
          <DataTable
            :columns="userDetailColumns"
            :data="userDetails"
            :loading="userDetailsDialog.loading"
            row-key="user_id"
            :sticky-actions-column="false"
            :estimate-row-height="64"
          >
            <template #cell-email="{ row }">
              <div class="max-w-64 truncate">
                <div class="font-medium text-gray-900 dark:text-white">{{ row.email }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">#{{ row.user_id }} {{ row.user_name || '-' }}</div>
              </div>
            </template>
            <template #cell-balance="{ row }">{{ formatUSD(row.balance) }}</template>
            <template #cell-total_recharged="{ row }">{{ formatUSD(row.total_recharged) }}</template>
            <template #cell-period_usage_cost="{ row }">{{ formatUSD(row.period_usage_cost) }}</template>
            <template #cell-paid_order_amount="{ row }">{{ formatMoney(row.paid_order_amount) }}</template>
            <template #cell-last_active_at="{ row }">{{ formatOptionalDateTime(row.last_active_at) }}</template>
          </DataTable>
          <Pagination
            v-if="userDetailsPagination.total > 0"
            :page="userDetailsPagination.page"
            :total="userDetailsPagination.total"
            :page-size="userDetailsPagination.page_size"
            @update:page="handleUserDetailsPageChange"
            @update:pageSize="handleUserDetailsPageSizeChange"
          />
        </div>
      </BaseDialog>

      <BaseDialog
        :show="recipientDialog.show"
        :title="t('admin.operations.marketing.recipientDialogTitle')"
        width="extra-wide"
        @close="recipientDialog.show = false"
      >
        <div class="space-y-4">
          <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
            <input v-model="recipientFilters.keyword" type="search" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.keywordPlaceholder')" @keyup.enter="reloadRecipients" />
            <select v-model="recipientFilters.audience" class="input input-sm text-xs">
              <option v-for="option in marketingAudienceOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
            <select v-model="recipientFilters.status" class="input input-sm text-xs">
              <option v-for="option in marketingStatusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
            <input v-model.number="recipientFilters.active_days" type="number" min="1" max="365" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.activeDays')" />
            <input v-model="recipientFilters.min_balance" type="number" min="0" step="0.01" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.minBalance')" />
            <input v-model="recipientFilters.max_balance" type="number" min="0" step="0.01" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.maxBalance')" />
            <input v-model="recipientFilters.min_total_recharged" type="number" min="0" step="0.01" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.minTotalRecharged')" />
            <input v-model="recipientFilters.max_total_recharged" type="number" min="0" step="0.01" class="input input-sm text-xs" :placeholder="t('admin.operations.marketing.maxTotalRecharged')" />
          </div>
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.operations.marketing.selectedUsersHint', { count: selectedMarketingUserIds.length }) }}
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="recipientDialog.loading" @click="reloadRecipients">
                <Icon name="search" size="sm" />
                {{ t('common.search') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="toggleCurrentRecipientPage">
                <Icon name="check" size="sm" />
                {{ allCurrentRecipientsSelected ? t('admin.operations.marketing.unselectPage') : t('admin.operations.marketing.selectPage') }}
              </button>
            </div>
          </div>
          <DataTable
            :columns="recipientColumns"
            :data="recipients"
            :loading="recipientDialog.loading"
            row-key="user_id"
            :sticky-actions-column="false"
            :estimate-row-height="60"
          >
            <template #cell-select="{ row }">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="isRecipientSelected(row.user_id)"
                @click.stop
                @change="toggleRecipient(row.user_id)"
              />
            </template>
            <template #cell-email="{ row }">
              <div class="max-w-64 truncate">
                <div class="font-medium text-gray-900 dark:text-white">{{ row.email }}</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">#{{ row.user_id }} {{ row.user_name || '-' }}</div>
              </div>
            </template>
            <template #cell-balance="{ row }">{{ formatUSD(row.balance) }}</template>
            <template #cell-total_recharged="{ row }">{{ formatUSD(row.total_recharged) }}</template>
            <template #cell-last_active_at="{ row }">{{ formatOptionalDateTime(row.last_active_at) }}</template>
          </DataTable>
          <Pagination
            v-if="recipientPagination.total > 0"
            :page="recipientPagination.page"
            :total="recipientPagination.total"
            :page-size="recipientPagination.page_size"
            @update:page="handleRecipientPageChange"
            @update:pageSize="handleRecipientPageSizeChange"
          />
        </div>
        <template #footer>
          <div class="flex items-center justify-end gap-2">
            <button type="button" class="btn btn-secondary" @click="recipientDialog.show = false">{{ t('common.cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="applyRecipientSelection">
              {{ t('admin.operations.marketing.applySelection', { count: selectedMarketingUserIds.length }) }}
            </button>
          </div>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  OperationsFunnelResponse,
  OperationsFunnelStep,
  OperationsMarketingEmailRecord,
  OperationsMarketingEmailRequest,
  OperationsMarketingEmailRecipient,
  OperationsMarketingEmailResult,
  OperationsUserDetail,
  OperationsUserSegment,
} from '@/api/admin/dashboard'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

type ShortcutKey = '7d' | '30d' | '90d' | 'custom'
type DateField = 'created_at' | 'paid_at'
type IconName = 'userPlus' | 'chart' | 'creditCard' | 'dollar'
type MarketingAudience = 'all' | 'active' | 'inactive'
type MarketingStatus = 'active' | 'all' | 'disabled'
type MarketingBodyFormat = 'plain' | 'html'

interface MarketingForm {
  subject: string
  body: string
  body_format: MarketingBodyFormat
  audience: MarketingAudience
  status: MarketingStatus
  active_days: number
  min_balance: string
  max_balance: string
  min_total_recharged: string
  max_total_recharged: string
  limit: number
}

interface RecipientFilters {
  keyword: string
  audience: MarketingAudience
  status: MarketingStatus
  active_days: number
  min_balance: string
  max_balance: string
  min_total_recharged: string
  max_total_recharged: string
}

const { t } = useI18n()
const appStore = useAppStore()

function fmtDate(date: Date): string {
  return date.toISOString().slice(0, 10)
}

function daysAgo(days: number): string {
  const today = new Date()
  return fmtDate(new Date(today.getFullYear(), today.getMonth(), today.getDate() - days + 1))
}

const startDate = ref(daysAgo(30))
const endDate = ref(fmtDate(new Date()))
const activeShortcut = ref<ShortcutKey>('30d')
const loading = ref(false)
const funnel = ref<OperationsFunnelResponse | null>(null)
const marketingPreviewLoading = ref(false)
const marketingSending = ref(false)
const marketingResult = ref<OperationsMarketingEmailResult | null>(null)
const marketingForm = reactive<MarketingForm>({
  subject: '',
  body: '',
  body_format: 'plain',
  audience: 'all',
  status: 'active',
  active_days: 30,
  min_balance: '',
  max_balance: '',
  min_total_recharged: '',
  max_total_recharged: '',
  limit: 50,
})
const userDetailsDialog = reactive({
  show: false,
  title: '',
  segment: 'all' as OperationsUserSegment,
  loading: false,
})
const userDetails = ref<OperationsUserDetail[]>([])
const userDetailsPagination = reactive({ page: 1, page_size: 20, total: 0 })
const selectedMarketingUserIds = ref<number[]>([])
const recipientDialog = reactive({ show: false, loading: false })
const recipientFilters = reactive<RecipientFilters>({
  keyword: '',
  audience: 'all',
  status: 'active',
  active_days: 30,
  min_balance: '',
  max_balance: '',
  min_total_recharged: '',
  max_total_recharged: '',
})
const recipients = ref<OperationsMarketingEmailRecipient[]>([])
const recipientPagination = reactive({ page: 1, page_size: 20, total: 0 })
const marketingRecords = ref<OperationsMarketingEmailRecord[]>([])
const marketingRecordsLoading = ref(false)

const shortcuts = computed(() => [
  { key: '7d' as ShortcutKey, label: t('dates.last7Days') },
  { key: '30d' as ShortcutKey, label: t('dates.last30Days') },
  { key: '90d' as ShortcutKey, label: t('admin.operations.rangeDays', { days: 90 }) },
])

const marketingAudienceOptions = computed<{ value: MarketingAudience; label: string }[]>(() => [
  { value: 'all', label: t('admin.operations.marketing.audienceAll') },
  { value: 'active', label: t('admin.operations.marketing.audienceActive') },
  { value: 'inactive', label: t('admin.operations.marketing.audienceInactive') },
])

const marketingStatusOptions = computed<{ value: MarketingStatus; label: string }[]>(() => [
  { value: 'active', label: t('admin.operations.marketing.statusActive') },
  { value: 'all', label: t('admin.operations.marketing.statusAll') },
  { value: 'disabled', label: t('admin.operations.marketing.statusDisabled') },
])

const marketingBodyFormatOptions = computed<{ value: MarketingBodyFormat; label: string }[]>(() => [
  { value: 'plain', label: t('admin.operations.marketing.formatPlain') },
  { value: 'html', label: t('admin.operations.marketing.formatHtml') },
])

const userDetailColumns = computed<Column[]>(() => [
  { key: 'email', label: t('admin.operations.detail.user') },
  { key: 'status', label: t('admin.operations.detail.status') },
  { key: 'balance', label: t('admin.operations.detail.balance') },
  { key: 'total_recharged', label: t('admin.operations.detail.totalRecharged') },
  { key: 'period_requests', label: t('admin.operations.detail.periodRequests') },
  { key: 'period_usage_cost', label: t('admin.operations.detail.periodUsageCost') },
  { key: 'paid_order_amount', label: t('admin.operations.detail.paidOrderAmount') },
  { key: 'active_subscription_count', label: t('admin.operations.detail.activeSubscriptions') },
  { key: 'last_active_at', label: t('admin.operations.detail.lastActiveAt') },
])

const recipientColumns = computed<Column[]>(() => [
  { key: 'select', label: '' },
  { key: 'email', label: t('admin.operations.detail.user') },
  { key: 'status', label: t('admin.operations.detail.status') },
  { key: 'balance', label: t('admin.operations.detail.balance') },
  { key: 'total_recharged', label: t('admin.operations.detail.totalRecharged') },
  { key: 'last_active_at', label: t('admin.operations.detail.lastActiveAt') },
])

const stepLabels: Record<string, string> = {
  registered: 'admin.operations.registeredUsers',
  created_key: 'admin.operations.createdKeyUsers',
  active: 'admin.operations.activeUsers',
  paying: 'admin.operations.payingUsers',
}

const stepBadges = [
  'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
  'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
]

const localizedSteps = computed(() => {
  const steps = funnel.value?.steps ?? []
  const maxCount = Math.max(...steps.map((step) => step.count), 1)
  return steps.map((step, index) => ({
    ...step,
    index,
    label: t(stepLabels[step.key] || step.label),
    badgeClass: stepBadges[index] || stepBadges[0],
    barWidth: `${Math.max(2, Math.round((step.count / maxCount) * 100))}%`,
  }))
})

const summaryMetrics = computed<{ key: string; label: string; value: string; hint: string; icon: IconName; iconBg: string; iconColor: string }[]>(() => {
  const steps = stepMap(funnel.value?.steps ?? [])
  const revenue = funnel.value?.revenue
  return [
    {
      key: 'registered',
      label: t('admin.operations.registeredUsers'),
      value: formatNumber(steps.registered?.count ?? 0),
      hint: t('admin.operations.rangeDays', { days: funnel.value?.range_days ?? 0 }),
      icon: 'userPlus',
      iconBg: 'bg-blue-100 dark:bg-blue-900/30',
      iconColor: 'text-blue-600 dark:text-blue-400',
    },
    {
      key: 'active',
      label: t('admin.operations.activeUsers'),
      value: formatNumber(steps.active?.count ?? 0),
      hint: `${t('admin.operations.overallConversion')} ${formatPercent(steps.active?.overall_conversion_rate ?? 0)}`,
      icon: 'chart',
      iconBg: 'bg-emerald-100 dark:bg-emerald-900/30',
      iconColor: 'text-emerald-600 dark:text-emerald-400',
    },
    {
      key: 'paying',
      label: t('admin.operations.payingUsers'),
      value: formatNumber(steps.paying?.count ?? 0),
      hint: `${t('admin.operations.overallConversion')} ${formatPercent(steps.paying?.overall_conversion_rate ?? 0)}`,
      icon: 'creditCard',
      iconBg: 'bg-purple-100 dark:bg-purple-900/30',
      iconColor: 'text-purple-600 dark:text-purple-400',
    },
    {
      key: 'revenue',
      label: t('admin.operations.totalRevenue'),
      value: formatMoney(revenue?.total_revenue ?? 0),
      hint: `${formatNumber(revenue?.paid_orders ?? 0)} ${t('admin.operations.paidOrders')}`,
      icon: 'dollar',
      iconBg: 'bg-amber-100 dark:bg-amber-900/30',
      iconColor: 'text-amber-600 dark:text-amber-400',
    },
  ]
})

const userSummaryItems = computed(() => {
  const users = funnel.value?.users
  return [
    { key: 'total', label: t('admin.operations.totalUsers'), value: formatNumber(users?.total_users ?? 0), segment: 'all' as OperationsUserSegment },
    { key: 'active', label: t('admin.operations.allActiveUsers'), value: formatNumber(users?.active_users ?? 0), segment: 'active' as OperationsUserSegment },
    { key: 'inactive', label: t('admin.operations.inactiveUsers'), value: formatNumber(users?.inactive_users ?? 0), segment: 'inactive' as OperationsUserSegment },
    { key: 'activeRate', label: t('admin.operations.activeRate'), value: formatPercent(users?.active_rate ?? 0) },
  ]
})

const creditSummaryItems = computed(() => {
  const credits = funnel.value?.credits
  return [
    {
      key: 'recharge',
      label: t('admin.operations.totalRechargeAmount'),
      value: formatUSD(credits?.total_recharge_amount ?? 0),
      segment: 'all' as OperationsUserSegment,
      details: [
        { label: t('admin.operations.balanceRechargeAmount'), value: formatUSD(credits?.balance_recharge_amount ?? 0) },
        { label: t('admin.operations.subscriptionRechargeAmount'), value: formatUSD(credits?.subscription_recharge_amount ?? 0) },
      ],
    },
    {
      key: 'remaining',
      label: t('admin.operations.totalRemainingAmount'),
      value: formatUSD(credits?.total_remaining_amount ?? 0),
      segment: 'all' as OperationsUserSegment,
      details: [
        { label: t('admin.operations.balanceRechargeRemaining'), value: formatUSD(credits?.balance_recharge_remaining ?? 0) },
        { label: t('admin.operations.subscriptionRemaining'), value: formatUSD(credits?.subscription_remaining ?? 0) },
        { label: t('admin.operations.giftedRemaining'), value: formatUSD(credits?.gifted_remaining ?? 0) },
      ],
    },
  ]
})

const subscriptionSummaryItems = computed(() => {
  const subscriptions = funnel.value?.subscriptions
  return [
    { key: 'active', label: t('admin.operations.activeSubscriptions'), value: formatNumber(subscriptions?.active_subscriptions ?? 0) },
    { key: 'activeUsers', label: t('admin.operations.activeSubscriptionUsers'), value: formatNumber(subscriptions?.active_subscription_users ?? 0) },
    { key: 'limited', label: t('admin.operations.limitedSubscriptions'), value: formatNumber(subscriptions?.limited_subscriptions ?? 0) },
    { key: 'daily', label: t('admin.operations.dailyRemaining'), value: formatUSD(subscriptions?.daily_remaining_usd ?? 0) },
    { key: 'weekly', label: t('admin.operations.weeklyRemaining'), value: formatUSD(subscriptions?.weekly_remaining_usd ?? 0) },
    { key: 'monthly', label: t('admin.operations.monthlyRemaining'), value: formatUSD(subscriptions?.monthly_remaining_usd ?? 0) },
  ]
})

const breakdownGroups = computed(() => {
  const breakdown = funnel.value?.breakdown
  if (!breakdown) return []
  return [
    {
      key: 'users',
      label: t('admin.operations.breakdownUsers'),
      items: (breakdown.users || []).map((item) => ({
        key: item.key,
        label: localizedBreakdownLabel(item.key, item.label),
        value: item.percent ? `${formatNumber(item.count)} / ${formatPercent(item.percent)}` : formatNumber(item.count),
        segment: breakdownSegment(item.key),
      })),
    },
    {
      key: 'credits',
      label: t('admin.operations.breakdownCredits'),
      items: (breakdown.credits || []).map((item) => ({
        key: item.key,
        label: localizedBreakdownLabel(item.key, item.label),
        value: formatUSD(item.amount),
        segment: breakdownSegment(item.key),
      })),
    },
    {
      key: 'revenue',
      label: t('admin.operations.breakdownRevenue'),
      items: (breakdown.revenue || []).map((item) => ({
        key: item.key,
        label: localizedBreakdownLabel(item.key, item.label),
        value: item.amount ? `${formatMoney(item.amount)} / ${formatNumber(item.count)}` : formatNumber(item.count),
        segment: breakdownSegment(item.key),
      })),
    },
    {
      key: 'subscriptions',
      label: t('admin.operations.breakdownSubscriptions'),
      items: (breakdown.subscriptions || []).map((item) => ({
        key: item.key,
        label: localizedBreakdownLabel(`quota_${item.key}`, item.label),
        value: `${formatUSD(item.remaining_usd)} / ${formatPercent(item.utilization_rate)}`,
        segment: 'subscription' as OperationsUserSegment,
      })),
    },
  ].filter((group) => group.items.length > 0)
})

const revenueItems = computed(() => {
  const revenue = funnel.value?.revenue
  return [
    { key: 'total', label: t('admin.operations.totalRevenue'), value: formatMoney(revenue?.total_revenue ?? 0) },
    { key: 'paidOrders', label: t('admin.operations.paidOrders'), value: formatNumber(revenue?.paid_orders ?? 0) },
    { key: 'payingUsers', label: t('admin.operations.payingUsersInPeriod'), value: formatNumber(revenue?.paying_users ?? 0) },
    { key: 'avg', label: t('admin.operations.averageOrderAmount'), value: formatMoney(revenue?.average_order_amount ?? 0) },
    { key: 'balance', label: t('admin.operations.balanceRevenue'), value: formatMoney(revenue?.balance_revenue ?? 0) },
    { key: 'balanceOrders', label: t('admin.operations.balanceOrders'), value: formatNumber(revenue?.balance_orders ?? 0) },
    { key: 'subscription', label: t('admin.operations.subscriptionRevenue'), value: formatMoney(revenue?.subscription_revenue ?? 0) },
    { key: 'subscriptionOrders', label: t('admin.operations.subscriptionOrders'), value: formatNumber(revenue?.subscription_orders ?? 0) },
  ]
})

const signalItems = computed(() => {
  const signals = funnel.value?.signals
  return [
    { key: 'pending', status: 'PENDING', label: t('admin.operations.pendingOrders'), value: signals?.pending_orders ?? 0, dotClass: 'bg-yellow-500' },
    { key: 'failed', status: 'FAILED', label: t('admin.operations.failedOrders'), value: signals?.failed_orders ?? 0, dotClass: 'bg-red-500' },
    { key: 'refund', status: 'REFUND_REQUESTED', label: t('admin.operations.refundRequestedOrders'), value: signals?.refund_requested_orders ?? 0, dotClass: 'bg-purple-500' },
    { key: 'expired', status: 'EXPIRED', label: t('admin.operations.expiredOrders'), value: signals?.expired_orders ?? 0, dotClass: 'bg-gray-400' },
    { key: 'cancelled', status: 'CANCELLED', label: t('admin.operations.cancelledOrders'), value: signals?.cancelled_orders ?? 0, dotClass: 'bg-slate-500' },
  ]
})

function stepMap(steps: OperationsFunnelStep[]) {
  return Object.fromEntries(steps.map((step) => [step.key, step])) as Record<string, OperationsFunnelStep | undefined>
}

function applyShortcut(key: ShortcutKey) {
  activeShortcut.value = key
  if (key !== 'custom') {
    const days = Number.parseInt(key, 10)
    startDate.value = daysAgo(days)
    endDate.value = fmtDate(new Date())
  }
  loadFunnel()
}

function onCustomRange() {
  activeShortcut.value = 'custom'
  loadFunnel()
}

function ordersLink(status: string, dateField: DateField = 'created_at') {
  return {
    path: '/admin/orders',
    query: {
      status,
      date_field: dateField,
      start_date: startDate.value,
      end_date: endDate.value,
    },
  }
}

function parseOptionalAmount(value: string): number | undefined {
  const trimmed = String(value || '').trim()
  if (!trimmed) return undefined
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : undefined
}

function buildMarketingEmailPayload(dryRun: boolean): OperationsMarketingEmailRequest | null {
  const subject = marketingForm.subject.trim()
  const body = marketingForm.body.trim()
  if (!subject || !body) {
    appStore.showError(t('admin.operations.marketing.validationRequired'))
    return null
  }

  const minBalance = parseOptionalAmount(marketingForm.min_balance)
  const maxBalance = parseOptionalAmount(marketingForm.max_balance)
  const minTotalRecharged = parseOptionalAmount(marketingForm.min_total_recharged)
  const maxTotalRecharged = parseOptionalAmount(marketingForm.max_total_recharged)
  if (
    (marketingForm.min_balance.trim() && minBalance === undefined) ||
    (marketingForm.max_balance.trim() && maxBalance === undefined) ||
    (marketingForm.min_total_recharged.trim() && minTotalRecharged === undefined) ||
    (marketingForm.max_total_recharged.trim() && maxTotalRecharged === undefined)
  ) {
    appStore.showError(t('admin.operations.marketing.validationBalance'))
    return null
  }
  if (minBalance !== undefined && maxBalance !== undefined && minBalance > maxBalance) {
    appStore.showError(t('admin.operations.marketing.validationBalanceRange'))
    return null
  }
  if (minTotalRecharged !== undefined && maxTotalRecharged !== undefined && minTotalRecharged > maxTotalRecharged) {
    appStore.showError(t('admin.operations.marketing.validationRechargeRange'))
    return null
  }

  const limit = Number(marketingForm.limit)
  const activeDays = Number(marketingForm.active_days)
  if (!Number.isFinite(limit) || limit <= 0 || !Number.isFinite(activeDays) || activeDays <= 0) {
    appStore.showError(t('admin.operations.marketing.validationLimit'))
    return null
  }

  const payload: OperationsMarketingEmailRequest = {
    subject,
    body,
    body_format: marketingForm.body_format,
    audience: marketingForm.audience,
    status: marketingForm.status,
    active_days: Math.floor(activeDays),
    limit: selectedMarketingUserIds.value.length ? Math.min(selectedMarketingUserIds.value.length, 100) : Math.min(Math.floor(limit), 100),
    dry_run: dryRun,
    confirm: !dryRun,
  }
  if (minBalance !== undefined) payload.min_balance = minBalance
  if (maxBalance !== undefined) payload.max_balance = maxBalance
  if (minTotalRecharged !== undefined) payload.min_total_recharged = minTotalRecharged
  if (maxTotalRecharged !== undefined) payload.max_total_recharged = maxTotalRecharged
  if (selectedMarketingUserIds.value.length) payload.user_ids = selectedMarketingUserIds.value.slice(0, 100)
  return payload
}

async function previewMarketingEmail() {
  const payload = buildMarketingEmailPayload(true)
  if (!payload) return
  marketingPreviewLoading.value = true
  try {
    marketingResult.value = await adminAPI.dashboard.sendOperationsMarketingEmail(payload)
    appStore.showSuccess(t('admin.operations.marketing.previewLoaded', { count: marketingResult.value.targeted }))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.operations.errors', t('admin.operations.marketing.previewFailed')))
  } finally {
    marketingPreviewLoading.value = false
  }
}

async function sendMarketingEmail() {
  const payload = buildMarketingEmailPayload(false)
  if (!payload) return
  const expectedCount = marketingResult.value?.targeted ?? payload.limit ?? 0
  if (!window.confirm(t('admin.operations.marketing.confirmSend', { count: expectedCount }))) {
    return
  }
  marketingSending.value = true
  try {
    marketingResult.value = await adminAPI.dashboard.sendOperationsMarketingEmail(payload)
    appStore.showSuccess(t('admin.operations.marketing.sendFinished', {
      sent: marketingResult.value.sent,
      failed: marketingResult.value.failed,
    }))
    loadMarketingRecords()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.operations.errors', t('admin.operations.marketing.sendFailed')))
  } finally {
    marketingSending.value = false
  }
}

function localizedBreakdownLabel(key: string, fallback: string): string {
  const labels: Record<string, string> = {
    active_period: t('admin.operations.breakdownLabels.activePeriod'),
    inactive_period: t('admin.operations.breakdownLabels.inactivePeriod'),
    status_active: t('admin.operations.breakdownLabels.statusActive'),
    status_disabled: t('admin.operations.breakdownLabels.statusDisabled'),
    recharged: t('admin.operations.breakdownLabels.recharged'),
    balance_recharge: t('admin.operations.breakdownLabels.balanceRecharge'),
    remaining_balance: t('admin.operations.breakdownLabels.remainingBalance'),
    gifted: t('admin.operations.breakdownLabels.gifted'),
    consumed: t('admin.operations.breakdownLabels.consumed'),
    balance: t('admin.operations.breakdownLabels.balanceRevenue'),
    subscription: t('admin.operations.breakdownLabels.subscriptionRevenue'),
    subscription_purchase_users: t('admin.operations.breakdownLabels.subscriptionPurchaseUsers'),
    quota_daily: t('admin.operations.breakdownLabels.quotaDaily'),
    quota_weekly: t('admin.operations.breakdownLabels.quotaWeekly'),
    quota_monthly: t('admin.operations.breakdownLabels.quotaMonthly'),
  }
  return labels[key] || fallback
}

function breakdownSegment(key: string): OperationsUserSegment | undefined {
  const map: Record<string, OperationsUserSegment> = {
    active_period: 'active',
    inactive_period: 'inactive',
    status_active: 'all',
    status_disabled: 'all',
    recharged: 'recharge',
    balance_recharge: 'recharge',
    remaining_balance: 'balance',
    gifted: 'recharge',
    balance: 'recharge',
    subscription: 'subscription',
    subscription_purchase_users: 'subscription',
  }
  return map[key]
}

async function openUserDetails(segment: OperationsUserSegment, title: string) {
  userDetailsDialog.segment = segment
  userDetailsDialog.title = title
  userDetailsDialog.show = true
  userDetailsPagination.page = 1
  await loadUserDetails()
}

async function loadUserDetails() {
  userDetailsDialog.loading = true
  try {
    const res = await adminAPI.dashboard.getOperationsUserDetails({
      segment: userDetailsDialog.segment,
      start_date: startDate.value,
      end_date: endDate.value,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      page: userDetailsPagination.page,
      page_size: userDetailsPagination.page_size,
    })
    userDetails.value = res.items
    userDetailsPagination.total = res.total
    userDetailsPagination.page = res.page
    userDetailsPagination.page_size = res.page_size
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.operations.errors', t('admin.operations.detail.loadFailed')))
  } finally {
    userDetailsDialog.loading = false
  }
}

function handleUserDetailsPageChange(page: number) {
  userDetailsPagination.page = page
  loadUserDetails()
}

function handleUserDetailsPageSizeChange(pageSize: number) {
  userDetailsPagination.page = 1
  userDetailsPagination.page_size = pageSize
  loadUserDetails()
}

function openRecipientDialog() {
  recipientDialog.show = true
  recipientPagination.page = 1
  loadRecipients()
}

function buildRecipientParams() {
  const minBalance = parseOptionalAmount(recipientFilters.min_balance)
  const maxBalance = parseOptionalAmount(recipientFilters.max_balance)
  const minTotalRecharged = parseOptionalAmount(recipientFilters.min_total_recharged)
  const maxTotalRecharged = parseOptionalAmount(recipientFilters.max_total_recharged)
  if (
    (recipientFilters.min_balance.trim() && minBalance === undefined) ||
    (recipientFilters.max_balance.trim() && maxBalance === undefined) ||
    (recipientFilters.min_total_recharged.trim() && minTotalRecharged === undefined) ||
    (recipientFilters.max_total_recharged.trim() && maxTotalRecharged === undefined)
  ) {
    appStore.showError(t('admin.operations.marketing.validationBalance'))
    return null
  }
  if (minBalance !== undefined && maxBalance !== undefined && minBalance > maxBalance) {
    appStore.showError(t('admin.operations.marketing.validationBalanceRange'))
    return null
  }
  if (minTotalRecharged !== undefined && maxTotalRecharged !== undefined && minTotalRecharged > maxTotalRecharged) {
    appStore.showError(t('admin.operations.marketing.validationRechargeRange'))
    return null
  }

  const params: Record<string, string | number> = {
    audience: recipientFilters.audience,
    status: recipientFilters.status,
    active_days: Math.max(1, Number(recipientFilters.active_days) || 30),
    page: recipientPagination.page,
    page_size: recipientPagination.page_size,
  }
  if (recipientFilters.keyword.trim()) params.keyword = recipientFilters.keyword.trim()
  if (minBalance !== undefined) params.min_balance = minBalance
  if (maxBalance !== undefined) params.max_balance = maxBalance
  if (minTotalRecharged !== undefined) params.min_total_recharged = minTotalRecharged
  if (maxTotalRecharged !== undefined) params.max_total_recharged = maxTotalRecharged
  return params
}

async function loadRecipients() {
  const params = buildRecipientParams()
  if (!params) return
  recipientDialog.loading = true
  try {
    const res = await adminAPI.dashboard.getOperationsMarketingRecipients(params)
    recipients.value = res.items
    recipientPagination.total = res.total
    recipientPagination.page = res.page
    recipientPagination.page_size = res.page_size
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.operations.errors', t('admin.operations.marketing.recipientsLoadFailed')))
  } finally {
    recipientDialog.loading = false
  }
}

function reloadRecipients() {
  recipientPagination.page = 1
  loadRecipients()
}

function handleRecipientPageChange(page: number) {
  recipientPagination.page = page
  loadRecipients()
}

function handleRecipientPageSizeChange(pageSize: number) {
  recipientPagination.page = 1
  recipientPagination.page_size = pageSize
  loadRecipients()
}

function isRecipientSelected(userId: number): boolean {
  return selectedMarketingUserIds.value.includes(userId)
}

function toggleRecipient(userId: number) {
  if (isRecipientSelected(userId)) {
    selectedMarketingUserIds.value = selectedMarketingUserIds.value.filter((id) => id !== userId)
    return
  }
  if (selectedMarketingUserIds.value.length >= 100) {
    appStore.showError(t('admin.operations.marketing.validationUserLimit'))
    return
  }
  selectedMarketingUserIds.value = [...selectedMarketingUserIds.value, userId]
}

const allCurrentRecipientsSelected = computed(() => (
  recipients.value.length > 0 && recipients.value.every((item) => selectedMarketingUserIds.value.includes(item.user_id))
))

function toggleCurrentRecipientPage() {
  const pageIds = recipients.value.map((item) => item.user_id)
  if (allCurrentRecipientsSelected.value) {
    selectedMarketingUserIds.value = selectedMarketingUserIds.value.filter((id) => !pageIds.includes(id))
    return
  }
  const next = [...selectedMarketingUserIds.value]
  for (const id of pageIds) {
    if (next.includes(id)) continue
    if (next.length >= 100) {
      appStore.showError(t('admin.operations.marketing.validationUserLimit'))
      break
    }
    next.push(id)
  }
  selectedMarketingUserIds.value = next
}

function applyRecipientSelection() {
  marketingForm.limit = selectedMarketingUserIds.value.length || marketingForm.limit
  recipientDialog.show = false
}

function clearSelectedRecipients() {
  selectedMarketingUserIds.value = []
}

async function loadMarketingRecords() {
  marketingRecordsLoading.value = true
  try {
    const res = await adminAPI.dashboard.getOperationsMarketingEmailRecords({ page: 1, page_size: 5 })
    marketingRecords.value = res.items
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.operations.errors', t('admin.operations.marketing.recordsLoadFailed')))
  } finally {
    marketingRecordsLoading.value = false
  }
}

function marketingAudienceLabel(audience: string): string {
  switch (audience) {
    case 'active':
      return t('admin.operations.marketing.audienceActive')
    case 'inactive':
      return t('admin.operations.marketing.audienceInactive')
    default:
      return t('admin.operations.marketing.audienceAll')
  }
}

function formatNumber(value: number): string {
  return Number(value || 0).toLocaleString()
}

function formatPercent(value: number): string {
  return `${Number(value || 0).toFixed(2)}%`
}

function formatMoney(value: number): string {
  return `¥${Number(value || 0).toFixed(2)}`
}

function formatUSD(value: number): string {
  return `$${Number(value || 0).toFixed(2)}`
}

function formatDateTime(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function formatOptionalDateTime(value?: string): string {
  return value ? formatDateTime(value) : '-'
}

async function loadFunnel() {
  loading.value = true
  try {
    funnel.value = await adminAPI.dashboard.getOperationsFunnel({
      start_date: startDate.value,
      end_date: endDate.value,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    })
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.operations.errors', t('admin.operations.loadFailed')))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadFunnel()
  loadMarketingRecords()
})
</script>
