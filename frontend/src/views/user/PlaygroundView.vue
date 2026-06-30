<template>
  <AppLayout>
    <div class="-m-4 flex h-[calc(100vh-4rem)] min-h-[720px] overflow-hidden bg-gray-50 text-slate-900 dark:bg-dark-950 dark:text-white md:-m-6 lg:-m-8 lg:h-[calc(100vh-4rem)]">
      <aside class="hidden w-[320px] shrink-0 flex-col border-r border-slate-200 bg-white/80 px-3 py-4 dark:border-dark-800 dark:bg-dark-900/80 lg:flex">
        <div class="flex h-10 items-center gap-3 px-1">
          <button
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-lg text-slate-500 transition-colors hover:bg-white hover:text-slate-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('common.back')"
          >
            <Icon name="chevronLeft" size="sm" />
          </button>
          <h1 class="text-xl font-semibold tracking-normal text-slate-950 dark:text-white">
            {{ t('playground.experienceCenter') }}
          </h1>
        </div>

        <nav class="mt-7 space-y-3">
          <button
            v-for="option in modeOptions"
            :key="option.value"
            type="button"
            class="flex h-12 w-full items-center gap-3 rounded-lg border px-4 text-left text-base font-medium transition-colors"
            :class="mode === option.value
              ? 'border-sky-300 bg-sky-100 text-sky-600 shadow-sm dark:border-sky-700 dark:bg-sky-950/50 dark:text-sky-300'
              : 'border-transparent text-slate-500 hover:bg-white hover:text-slate-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white'"
            @click="selectMode(option.value)"
          >
            <Icon :name="option.icon" size="md" />
            <span>{{ option.label }}</span>
          </button>
        </nav>

        <div class="mt-9 flex min-h-0 flex-1 flex-col">
          <div class="flex items-center justify-between px-1">
            <p class="text-sm font-semibold text-slate-600 dark:text-dark-200">
              {{ t('playground.recentConversations') }}
              <span class="ml-1 text-slate-400">{{ filteredThreads.length }}</span>
            </p>
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="inline-flex items-center gap-1 text-xs font-medium text-slate-500 transition-colors hover:text-slate-900 dark:text-dark-300 dark:hover:text-white"
                :title="t('playground.importConversations')"
                @click="historyImportInput?.click()"
              >
                <Icon name="upload" size="xs" />
                {{ t('common.import') }}
              </button>
              <button
                type="button"
                class="inline-flex items-center gap-1 text-xs font-medium text-slate-500 transition-colors hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50 dark:text-dark-300 dark:hover:text-white"
                :title="t('playground.exportConversations')"
                :disabled="modeThreads.length === 0"
                @click="exportConversations"
              >
                <Icon name="download" size="xs" />
                {{ t('common.export') }}
              </button>
              <button
                type="button"
                class="inline-flex items-center gap-1 text-xs font-medium text-red-500 transition-colors hover:text-red-600"
                @click="clearHistory"
              >
                <Icon name="trash" size="xs" />
                {{ t('playground.clearHistory') }}
              </button>
              <input
                ref="historyImportInput"
                type="file"
                accept=".json,application/json"
                class="hidden"
                @change="handleConversationImportChange"
              >
            </div>
          </div>

          <div class="relative mt-4">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              v-model="historySearch"
              class="h-10 w-full rounded-lg border border-slate-200 bg-white pl-9 pr-3 text-sm text-slate-900 outline-none transition focus:border-sky-400 focus:ring-2 focus:ring-sky-100 dark:border-dark-700 dark:bg-dark-950 dark:text-white dark:focus:border-sky-500 dark:focus:ring-sky-900/40"
              :placeholder="t('playground.searchConversations')"
            />
          </div>

          <div class="mt-4 min-h-0 flex-1 overflow-y-auto pr-1">
            <div v-if="filteredThreads.length === 0" class="rounded-lg border border-dashed border-slate-200 bg-white/60 p-4 text-sm text-slate-500 dark:border-dark-700 dark:bg-dark-900/60 dark:text-dark-300">
              {{ t('playground.noRecentConversations') }}
            </div>

            <div v-else class="space-y-2">
              <button
                v-for="thread in filteredThreads"
                :key="thread.id"
                type="button"
                class="group w-full rounded-lg border p-3 text-left transition-colors"
                :class="thread.id === activeThreadId
                  ? 'border-sky-300 bg-sky-50 shadow-sm dark:border-sky-700 dark:bg-sky-950/40'
                  : 'border-transparent bg-white/80 hover:border-slate-200 hover:bg-white dark:bg-dark-950/60 dark:hover:border-dark-700 dark:hover:bg-dark-900'"
                @click="selectThread(thread.id)"
              >
                <div class="flex items-start gap-2">
                  <span
                    class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md"
                    :class="thread.id === activeThreadId ? 'bg-sky-500 text-white' : 'bg-slate-100 text-slate-400 dark:bg-dark-800 dark:text-dark-300'"
                  >
                    <Icon :name="modeIcon(thread.mode)" size="xs" />
                  </span>
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center justify-between gap-2">
                      <p class="truncate text-sm font-semibold text-slate-800 dark:text-white">
                        {{ thread.title }}
                      </p>
                      <span
                        v-if="thread.unreadCount"
                        class="flex h-5 min-w-5 items-center justify-center rounded-full bg-blue-600 px-1.5 text-[11px] font-semibold text-white"
                      >
                        {{ thread.unreadCount }}
                      </span>
                    </div>
                    <p class="mt-1 truncate text-xs text-slate-500 dark:text-dark-400">
                      {{ threadPreview(thread) }}
                    </p>
                    <p class="mt-2 text-xs text-slate-400 dark:text-dark-500">
                      {{ formatThreadDate(thread.updatedAt) }}
                    </p>
                  </div>
                  <button
                    type="button"
                    class="mt-7 rounded p-1 text-slate-300 opacity-0 transition hover:bg-red-50 hover:text-red-500 group-hover:opacity-100 dark:hover:bg-red-950/30"
                    :title="t('common.delete')"
                    @click.stop="deleteThread(thread.id)"
                  >
                    <Icon name="trash" size="xs" />
                  </button>
                </div>
              </button>
            </div>
          </div>
        </div>
      </aside>

      <section class="relative flex min-w-0 flex-1 flex-col bg-gray-50 dark:bg-dark-950">
        <header class="flex h-16 shrink-0 items-center justify-between border-b border-slate-200/80 px-4 dark:border-dark-800 md:px-7">
          <div class="flex items-center gap-3">
            <button
              type="button"
              class="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-100 px-4 text-sm font-semibold text-slate-700 transition hover:bg-slate-200 dark:bg-dark-800 dark:text-dark-100 dark:hover:bg-dark-700"
              @click="createThread(mode)"
            >
              <Icon name="chatBubble" size="sm" />
              {{ t('playground.newConversation') }}
            </button>
            <span class="hidden rounded-full bg-white px-3 py-1 text-xs font-medium text-slate-500 ring-1 ring-slate-200 dark:bg-dark-900 dark:text-dark-300 dark:ring-dark-700 md:inline-flex">
              {{ modeLabel(mode) }}
            </span>
          </div>

          <div class="flex items-center gap-2">
            <button
              type="button"
              class="flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
              :title="showAssistantDetails ? t('playground.hideDetails') : t('playground.showDetails')"
              @click="showAssistantDetails = !showAssistantDetails"
            >
              <Icon :name="showAssistantDetails ? 'eye' : 'eyeOff'" size="sm" />
            </button>
            <button
              type="button"
              class="flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
              :title="t('playground.settings')"
              @click="showSettings = true"
            >
              <Icon name="cog" size="sm" />
            </button>
          </div>
        </header>

        <main
          ref="messageScroller"
          class="min-h-0 flex-1 overflow-y-auto px-4 pt-8 md:px-10 xl:px-16"
          :style="{ paddingBottom: `${composerSpacerHeight}px` }"
        >
          <div v-if="!selectedKey && !loadingKeys" class="flex h-full items-center justify-center text-center">
            <div class="max-w-sm">
              <Icon name="key" size="xl" class="mx-auto text-slate-300 dark:text-dark-600" />
              <p class="mt-3 text-sm font-semibold text-slate-700 dark:text-dark-100">{{ t('playground.noKeys') }}</p>
            </div>
          </div>

          <div v-else-if="currentMessages.length === 0" class="flex h-full items-center justify-center text-center">
            <div class="max-w-md">
              <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-sky-50 text-sky-500 dark:bg-sky-950/40 dark:text-sky-300">
                <Icon :name="modeIcon(mode)" size="lg" />
              </div>
              <p class="mt-4 text-base font-semibold text-slate-800 dark:text-white">{{ t('playground.emptyResult') }}</p>
              <p class="mt-2 text-sm leading-6 text-slate-500 dark:text-dark-300">{{ modeHint }}</p>
            </div>
          </div>

          <div v-else class="mx-auto max-w-[1180px] space-y-7">
            <article
              v-for="message in currentMessages"
              :key="message.id"
              class="group/message flex"
              :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
            >
              <div
                v-if="message.role === 'assistant'"
                class="mr-4 mt-8 hidden h-8 w-8 shrink-0 items-center justify-center rounded-full bg-white shadow-sm ring-1 ring-slate-200 dark:bg-dark-900 dark:ring-dark-700 md:flex"
                :class="platformIconClass(messagePlatform(message) || '')"
              >
                <PlatformIcon :platform="messagePlatform(message)" size="sm" />
              </div>

              <div
                class="min-w-0"
                :class="message.role === 'user'
                  ? 'max-w-[78%]'
                  : message.images?.length
                    ? 'w-fit max-w-full sm:max-w-[760px]'
                    : 'max-w-[760px]'"
              >
                <div
                  v-if="showAssistantDetails || message.role === 'user'"
                  class="mb-2 flex items-center gap-2 text-xs"
                  :class="message.role === 'user' ? 'justify-end text-slate-500' : 'text-slate-400'"
                >
                  <span class="font-semibold">
                    {{ messageDisplayName(message) }}
                  </span>
                  <span>{{ formatMessageTime(message.createdAt) }}</span>
                </div>

                <div
                  class="rounded-lg border shadow-sm"
                  :class="[
                    message.images?.length ? 'inline-block p-2' : 'px-4 py-3',
                    message.role === 'user'
                      ? 'border-sky-300 bg-sky-50 text-sky-700 dark:border-sky-700 dark:bg-sky-950/50 dark:text-sky-200'
                      : message.error
                        ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300'
                        : 'border-slate-200 bg-white text-slate-800 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-100'
                  ]"
                >
                  <div
                    v-if="message.content"
                    class="playground-markdown break-words text-sm leading-7"
                    v-html="renderMessageMarkdown(message.content)"
                  ></div>
                  <div v-if="message.pending" class="mt-2 flex items-center gap-2 text-xs text-slate-400 dark:text-dark-400">
                    <span class="spinner h-3 w-3"></span>
                    {{ message.progress || t('playground.streaming') }}
                  </div>

                  <div v-if="message.attachments?.length" class="mt-3 flex flex-wrap gap-2">
                    <span
                      v-for="attachment in message.attachments"
                      :key="attachment.id"
                      class="inline-flex max-w-full items-center gap-1 rounded-full bg-slate-100 px-2 py-1 text-xs text-slate-500 ring-1 ring-slate-200 dark:bg-dark-800 dark:text-dark-300 dark:ring-dark-700"
                    >
                      <Icon name="document" size="xs" />
                      <span class="truncate">{{ attachment.name }}</span>
                    </span>
                  </div>

                  <div
                    v-if="message.images?.length"
                    class="mt-3 inline-grid max-w-full gap-3"
                    :class="message.images.length > 1 ? 'grid-cols-1 sm:grid-cols-2' : 'grid-cols-1'"
                  >
                    <figure
                      v-for="(image, index) in message.images"
                      :key="`${image.url}-${index}`"
                      class="playground-image-figure overflow-hidden rounded-lg border border-slate-200 bg-slate-50 dark:border-dark-700 dark:bg-dark-950"
                    >
                      <button
                        type="button"
                        class="group/image relative isolate block w-full bg-slate-100 text-left dark:bg-dark-900"
                        :title="t('playground.viewOriginalImage')"
                        @click="openImagePreview(image, index)"
                      >
                        <img
                          :src="image.url"
                          :alt="t('playground.generatedImageAlt', { n: index + 1 })"
                          class="block w-full object-contain"
                          loading="lazy"
                          decoding="async"
                        />
                        <span class="pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-black/0 opacity-0 transition group-hover/image:bg-black/20 group-hover/image:opacity-100">
                          <span class="inline-flex h-9 w-9 items-center justify-center rounded-full bg-white/90 text-slate-700 shadow-sm dark:bg-dark-900/90 dark:text-white">
                            <Icon name="eye" size="sm" />
                          </span>
                        </span>
                        <span class="pointer-events-none absolute inset-x-0 bottom-0 z-20 flex items-center justify-between gap-3 bg-gradient-to-t from-black/80 via-black/45 to-transparent px-3 pb-2 pt-8 text-[11px] font-semibold leading-none text-white">
                          <span class="min-w-0 truncate">{{ imageMessageSizeLabel(message) }}</span>
                          <span class="shrink-0">{{ formatImageGenerationDuration(message.durationMs) }}</span>
                        </span>
                      </button>
                      <figcaption v-if="image.revisedPrompt" class="border-t border-slate-200 p-3 text-xs leading-5 text-slate-500 dark:border-dark-700 dark:text-dark-400">
                        {{ image.revisedPrompt }}
                      </figcaption>
                    </figure>
                  </div>

                  <div v-if="message.images?.length && message.imageConfig" class="mt-3 flex justify-end">
                    <button
                      type="button"
                      class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 text-xs font-semibold text-slate-600 transition hover:border-sky-300 hover:bg-sky-50 hover:text-sky-600 dark:border-dark-700 dark:bg-dark-950 dark:text-dark-300 dark:hover:border-sky-700 dark:hover:bg-sky-950/40 dark:hover:text-sky-300"
                      @click="reuseImageConfig(message)"
                    >
                      <Icon name="refresh" size="xs" />
                      {{ t('playground.reuseImageConfig') }}
                    </button>
                  </div>
                </div>

                <div
                  class="mt-3 flex items-center gap-2 text-slate-400 opacity-0 transition-opacity group-hover/message:opacity-100 group-focus-within/message:opacity-100"
                  :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
                >
                  <button v-if="message.role === 'assistant' && message.content" class="rounded p-1 transition hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-800 dark:hover:text-white" :title="t('common.copy')" @click="copyText(message.content)">
                    <Icon name="copy" size="xs" />
                  </button>
                  <button v-if="message.role === 'assistant'" class="rounded p-1 transition hover:bg-slate-100 hover:text-slate-700 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-800 dark:hover:text-white" :title="t('playground.retry')" :disabled="running" @click="retryLastPrompt">
                    <Icon name="refresh" size="xs" />
                  </button>
                  <button class="rounded p-1 transition hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-950/30 dark:hover:text-red-300" :title="t('common.delete')" @click="deleteMessage(message.id)">
                    <Icon name="trash" size="xs" />
                  </button>
                </div>
              </div>

              <div
                v-if="message.role === 'user'"
                class="ml-4 mt-8 hidden h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-primary-500 to-primary-600 text-xs font-semibold text-white shadow-sm ring-1 ring-slate-200 dark:ring-dark-700 md:flex"
              >
                <img
                  v-if="userAvatarUrl"
                  :src="userAvatarUrl"
                  :alt="userDisplayName"
                  class="h-full w-full object-cover"
                >
                <span v-else>{{ userAvatarInitial }}</span>
              </div>
            </article>
          </div>
        </main>

        <footer ref="composerDock" class="pointer-events-none absolute bottom-0 left-0 right-0 z-40 bg-gradient-to-t from-gray-50 via-gray-50 to-transparent px-4 pb-4 pt-10 dark:from-dark-950 dark:via-dark-950 md:px-10 xl:px-16">
          <div class="pointer-events-auto mx-auto max-w-[1220px] rounded-lg border border-slate-200 bg-white/95 p-3 shadow-[0_20px_60px_-28px_rgba(15,23,42,0.35)] backdrop-blur dark:border-dark-700 dark:bg-dark-900/95">
            <div class="flex flex-wrap items-center gap-3 px-1 pb-3" :class="showComposerConfig ? '' : 'justify-end'">
              <template v-if="showComposerConfig">
                <Select
                  v-model="selectedKeyId"
                  class="playground-compact-select w-full sm:w-[180px]"
                  :options="keySelectOptions"
                  :placeholder="t('playground.selectKey')"
                  searchable
                >
                  <template #selected="{ option }">
                    <span v-if="option" class="playground-select-value">
                      <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                        <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                      </span>
                      <span class="truncate">{{ option.label }}</span>
                    </span>
                    <span v-else>{{ t('playground.selectKey') }}</span>
                  </template>
                  <template #option="{ option, selected }">
                    <div class="playground-select-option">
                      <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                        <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                      </span>
                      <span class="min-w-0 flex-1 truncate text-left">{{ option.label }}</span>
                      <Icon v-if="selected" name="check" size="sm" class="text-primary-500" />
                    </div>
                  </template>
                </Select>

                <Select
                  v-model="selectedEndpointBase"
                  class="playground-compact-select playground-endpoint-select"
                  :style="{ '--endpoint-select-width': endpointSelectWidth }"
                  :options="endpointSelectOptions"
                  :placeholder="t('playground.selectEndpoint')"
                  searchable
                />

                <Select
                  v-model="selectedModel"
                  class="playground-compact-select w-full sm:w-[240px]"
                  :options="modelSelectOptions"
                  :placeholder="t('playground.selectModel')"
                  searchable
                >
                  <template #selected="{ option }">
                    <span v-if="option" class="playground-select-value">
                      <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                        <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                      </span>
                      <span class="truncate">{{ option.label }}</span>
                    </span>
                    <span v-else>{{ t('playground.selectModel') }}</span>
                  </template>
                  <template #option="{ option, selected }">
                    <div class="playground-select-option">
                      <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                        <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                      </span>
                      <span class="min-w-0 flex-1 truncate text-left">{{ option.label }}</span>
                      <Icon v-if="selected" name="check" size="sm" class="text-primary-500" />
                    </div>
                  </template>
                </Select>

                <button
                  type="button"
                  class="ml-auto flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 transition hover:bg-white hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
                  :title="t('playground.refreshModels')"
                  :disabled="!selectedKey || loadingModels"
                  @click="loadModels"
                >
                  <Icon name="refresh" size="sm" :class="loadingModels ? 'animate-spin' : ''" />
                </button>
              </template>

              <button
                type="button"
                class="flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 transition hover:bg-white hover:text-slate-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
                :title="showComposerConfig ? t('playground.hideComposerConfig') : t('playground.showComposerConfig')"
                @click="showComposerConfig = !showComposerConfig"
              >
                <Icon :name="showComposerConfig ? 'eyeOff' : 'eye'" size="sm" />
              </button>
            </div>

            <div v-if="pendingAttachments.length" class="mb-2 flex flex-wrap gap-2 px-1">
              <span
                v-for="attachment in pendingAttachments"
                :key="attachment.id"
                class="inline-flex max-w-full items-center gap-1 rounded-full bg-white px-2 py-1 text-xs text-slate-600 ring-1 ring-slate-200 dark:bg-dark-800 dark:text-dark-300 dark:ring-dark-700"
              >
                <Icon name="document" size="xs" />
                <span class="truncate">{{ attachment.name }}</span>
                <button class="ml-1 text-slate-400 hover:text-red-500" @click="removeAttachment(attachment.id)">
                  <Icon name="x" size="xs" />
                </button>
              </span>
            </div>

            <div
              v-if="playgroundNotice"
              class="mb-3 flex items-start gap-2 rounded-lg border px-3 py-2 text-xs leading-5"
              :class="playgroundNotice.type === 'error'
                ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300'
                : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200'"
            >
              <Icon :name="playgroundNotice.type === 'error' ? 'exclamationCircle' : 'infoCircle'" size="sm" class="mt-0.5 shrink-0" />
              <span class="min-w-0 break-words">{{ playgroundNotice.message }}</span>
            </div>

            <div v-if="showComposerConfig && mode === 'image'" class="mb-2 flex flex-wrap items-center gap-2 px-1">
              <button
                type="button"
                class="image-option-button"
                @click="showImageSizeModal = true"
              >
                <Icon name="sparkles" size="xs" />
                <span>{{ t('playground.imageSizeButton') }}</span>
                <strong>{{ effectiveImageSize }}</strong>
              </button>

              <Select
                v-model="imageQuality"
                class="image-option-select w-[142px]"
                :options="imageQualitySelectOptions"
                :placeholder="t('playground.quality')"
                :searchable="false"
              />

              <Select
                v-model="outputFormat"
                class="image-option-select w-[168px]"
                :options="outputFormatSelectOptions"
                :placeholder="t('playground.outputFormat')"
                :searchable="false"
              />

              <button
                type="button"
                class="image-option-button"
                :disabled="optimizingPrompt"
                @click="showPromptOptimizerModal = true"
              >
                <Icon :name="optimizingPrompt ? 'refresh' : 'brain'" size="xs" :class="optimizingPrompt ? 'animate-spin' : ''" />
                <span>{{ optimizingPrompt ? t('playground.optimizingPrompt') : t('playground.optimizePrompt') }}</span>
                <strong>{{ imagePromptStyleLabel() }}</strong>
              </button>
            </div>

            <div
              class="relative flex min-h-[96px] items-stretch gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-950"
              :class="composerInputResizing ? 'select-none' : ''"
              :style="{ height: `${composerInputHeight}px` }"
            >
              <button
                type="button"
                class="absolute left-1/2 top-0 z-20 flex h-4 w-16 -translate-x-1/2 -translate-y-1/2 cursor-ns-resize items-center justify-center rounded-full border border-slate-200 bg-white text-slate-400 shadow-sm transition hover:border-sky-200 hover:bg-sky-50 hover:text-sky-500 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-400 dark:hover:border-sky-800 dark:hover:bg-sky-950/40 dark:hover:text-sky-300"
                :title="t('playground.resizeComposerInput')"
                @pointerdown="handleComposerInputResizePointerDown"
              >
                <span class="h-1 w-8 rounded-full bg-current"></span>
              </button>
              <button
                v-if="mode === 'image'"
                type="button"
                class="absolute right-16 top-3 z-10 flex h-8 w-8 items-center justify-center rounded-lg text-sky-500 transition hover:bg-sky-50 hover:text-sky-600 disabled:cursor-not-allowed disabled:opacity-50 dark:text-sky-300 dark:hover:bg-sky-950/40 dark:hover:text-sky-200"
                :title="optimizingPrompt ? t('playground.optimizingPrompt') : t('playground.optimizePrompt')"
                :disabled="optimizingPrompt || running || !draftPrompt.trim() || !selectedKey || !selectedEndpoint"
                @click="optimizeImagePrompt"
              >
                <Icon :name="optimizingPrompt ? 'refresh' : 'sparkles'" size="sm" :class="optimizingPrompt ? 'animate-spin' : ''" />
              </button>
              <textarea
                v-model="draftPrompt"
                class="min-h-[74px] flex-1 resize-none overflow-y-auto bg-transparent pr-10 text-sm leading-6 text-slate-900 outline-none placeholder:text-slate-400 dark:text-white"
                :placeholder="composerPlaceholder"
                @keydown.enter.exact.prevent="submitPrompt"
              ></textarea>

              <div class="flex shrink-0 items-end gap-2">
                <input ref="fileInput" type="file" multiple class="hidden" @change="handleFileChange" />
                <button
                  type="button"
                  class="flex h-10 w-10 items-center justify-center rounded-lg text-sky-500 transition hover:bg-sky-50 disabled:opacity-50 dark:hover:bg-sky-950/40"
                  :title="t('playground.uploadFile')"
                  @click="fileInput?.click()"
                >
                  <Icon name="upload" size="md" />
                </button>
                <button
                  v-if="running"
                  type="button"
                  class="flex h-10 w-10 items-center justify-center rounded-lg bg-slate-100 text-slate-500 transition hover:bg-slate-200 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700"
                  :title="t('playground.stop')"
                  @click="stopRun"
                >
                  <Icon name="x" size="md" />
                </button>
                <button
                  v-else
                  type="button"
                  class="flex h-10 w-10 items-center justify-center rounded-lg bg-sky-50 text-sky-500 transition hover:bg-sky-100 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-sky-950/40 dark:text-sky-300 dark:hover:bg-sky-900/50"
                  :title="t('playground.send')"
                  :disabled="running"
                  @click="submitPrompt"
                >
                  <Icon name="arrowRight" size="md" />
                </button>
              </div>
            </div>

            <p v-if="showComposerConfig" class="mt-2 px-1 text-xs text-slate-500 dark:text-dark-400">
              {{ t('playground.composerHint') }}
              <span v-if="selectedEndpointLabel" class="ml-2 break-all text-slate-400">{{ selectedEndpointLabel }}</span>
            </p>
          </div>
        </footer>
      </section>
    </div>

    <div v-if="showSettings" class="fixed inset-0 z-50 bg-black/30" @click.self="showSettings = false">
      <aside class="ml-auto flex h-full w-full max-w-md flex-col bg-white shadow-xl dark:bg-dark-900">
        <div class="flex items-center justify-between border-b border-slate-100 px-5 py-4 dark:border-dark-800">
          <h2 class="text-base font-semibold text-slate-900 dark:text-white">{{ t('playground.settings') }}</h2>
          <button class="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-800 dark:hover:text-white" @click="showSettings = false">
            <Icon name="x" size="md" />
          </button>
        </div>

        <div class="min-h-0 flex-1 space-y-5 overflow-y-auto p-5">
          <div>
            <label class="input-label">{{ t('playground.systemPrompt') }}</label>
            <textarea v-model="systemPrompt" class="input min-h-[120px] resize-y" :placeholder="t('playground.systemPromptPlaceholder')"></textarea>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="input-label">{{ t('playground.temperature') }}</label>
              <input v-model.number="temperature" class="input" type="number" min="0" max="2" step="0.1" />
            </div>
            <div>
              <label class="input-label">{{ t('playground.topP') }}</label>
              <input v-model.number="topP" class="input" type="number" min="0" max="1" step="0.05" />
            </div>
            <div>
              <label class="input-label">{{ t('playground.maxTokens') }}</label>
              <input v-model.number="maxTokens" class="input" type="number" min="1" step="1" />
            </div>
            <div>
              <label class="input-label">{{ t('playground.presencePenalty') }}</label>
              <input v-model.number="presencePenalty" class="input" type="number" min="-2" max="2" step="0.1" />
            </div>
            <div>
              <label class="input-label">{{ t('playground.frequencyPenalty') }}</label>
              <input v-model.number="frequencyPenalty" class="input" type="number" min="-2" max="2" step="0.1" />
            </div>
          </div>

          <div class="border-t border-slate-100 pt-5 dark:border-dark-800">
            <button
              type="button"
              class="flex w-full items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-3 py-3 text-left transition hover:bg-white dark:border-dark-700 dark:bg-dark-800 dark:hover:bg-dark-700"
              @click="showImageSizeModal = true"
            >
              <span>
                <span class="block text-sm font-semibold text-slate-800 dark:text-white">{{ t('playground.imageSize') }}</span>
                <span class="mt-1 block text-xs text-slate-500 dark:text-dark-300">{{ effectiveImageSize }}</span>
              </span>
              <Icon name="chevronRight" size="sm" class="text-slate-400" />
            </button>

            <div class="mt-3 grid grid-cols-2 gap-3">
              <div>
                <label class="input-label">{{ t('playground.imageCount') }}</label>
                <input v-model.number="imageCount" class="input" type="number" min="1" max="4" step="1" />
              </div>
              <div>
                <label class="input-label">{{ t('playground.quality') }}</label>
                <Select v-model="imageQuality" :options="imageQualitySettingOptions" :searchable="false" />
              </div>
            </div>
          </div>
        </div>
      </aside>
    </div>

    <div v-if="showImageSizeModal" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/30 p-4" @click.self="showImageSizeModal = false">
      <section class="w-full max-w-[560px] rounded-2xl bg-white p-6 shadow-2xl dark:bg-dark-900">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold text-slate-900 dark:text-white">{{ t('playground.setImageSize') }}</h2>
            <p class="mt-2 text-sm text-slate-400 dark:text-dark-300">
              {{ t('playground.currentImageSize') }}：{{ effectiveImageSize }}
            </p>
          </div>
          <button
            type="button"
            class="rounded-lg p-2 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-800 dark:hover:text-white"
            @click="showImageSizeModal = false"
          >
            <Icon name="x" size="md" />
          </button>
        </div>

        <div class="mt-7 grid grid-cols-3 rounded-xl bg-slate-100 p-1 dark:bg-dark-800">
          <button
            v-for="option in imageSizeModeOptions"
            :key="option.value"
            type="button"
            class="h-11 rounded-lg text-sm font-semibold transition"
            :class="imageSizeMode === option.value
              ? 'bg-white text-slate-900 shadow-sm dark:bg-dark-950 dark:text-white'
              : 'text-slate-500 hover:text-slate-900 dark:text-dark-300 dark:hover:text-white'"
            @click="imageSizeMode = option.value"
          >
            {{ option.label }}
          </button>
        </div>

        <div class="mt-8">
          <p class="text-sm font-semibold text-slate-400 dark:text-dark-300">{{ t('playground.baseResolution') }}</p>
          <div class="mt-3 grid grid-cols-3 gap-3">
            <button
              v-for="resolution in imageResolutionOptions"
              :key="resolution"
              type="button"
              class="h-12 rounded-xl border text-base font-medium transition"
              :class="imageResolution === resolution
                ? 'border-sky-500 bg-sky-50 text-sky-600 dark:bg-sky-950/40'
                : 'border-slate-200 text-slate-600 hover:bg-slate-50 dark:border-dark-700 dark:text-dark-200 dark:hover:bg-dark-800'"
              @click="imageResolution = resolution"
            >
              {{ resolution }}
            </button>
          </div>
        </div>

        <div class="mt-7" v-if="imageSizeMode === 'ratio'">
          <p class="text-sm font-semibold text-slate-400 dark:text-dark-300">{{ t('playground.imageRatio') }}</p>
          <div class="mt-3 grid grid-cols-4 gap-2.5">
            <button
              v-for="option in imageSizeOptions"
              :key="option.value"
              type="button"
              class="h-12 rounded-xl border text-base font-medium transition"
              :class="imageRatio === option.value
                ? 'border-sky-500 bg-sky-50 text-sky-600 dark:bg-sky-950/40'
                : 'border-slate-200 text-slate-600 hover:bg-slate-50 dark:border-dark-700 dark:text-dark-200 dark:hover:bg-dark-800'"
              @click="imageRatio = option.value"
            >
              {{ option.label }}
            </button>
          </div>
        </div>

        <div v-else-if="imageSizeMode === 'custom'" class="mt-7">
          <p class="text-sm font-semibold text-slate-400 dark:text-dark-300">{{ t('playground.customWidthHeight') }}</p>
          <div class="mt-3 grid grid-cols-2 gap-3">
            <input v-model.number="customImageWidth" type="number" min="256" max="4096" step="64" class="input" :placeholder="t('playground.width')" />
            <input v-model.number="customImageHeight" type="number" min="256" max="4096" step="64" class="input" :placeholder="t('playground.height')" />
          </div>
        </div>

        <div v-else class="mt-7 rounded-xl border border-dashed border-slate-200 p-4 text-sm text-slate-500 dark:border-dark-700 dark:text-dark-300">
          {{ t('playground.autoSizeHint') }}
        </div>

        <div class="mt-8 rounded-xl bg-slate-50 px-5 py-4 dark:bg-dark-950">
          <p class="text-sm font-semibold text-slate-400 dark:text-dark-300">{{ t('playground.willUse') }}</p>
          <p class="mt-2 text-2xl font-bold text-slate-800 dark:text-white">{{ effectiveImageSize }}</p>
        </div>

        <div class="mt-8 grid grid-cols-2 gap-3">
          <button type="button" class="h-12 rounded-xl bg-slate-100 text-sm font-semibold text-slate-600 transition hover:bg-slate-200 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700" @click="showImageSizeModal = false">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="h-12 rounded-xl bg-blue-500 text-sm font-semibold text-white transition hover:bg-blue-600" @click="showImageSizeModal = false">
            {{ t('common.confirm') }}
          </button>
        </div>
      </section>
    </div>

    <div v-if="showPromptOptimizerModal" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/30 p-4" @click.self="showPromptOptimizerModal = false">
      <section class="w-full max-w-[560px] rounded-2xl bg-white p-6 shadow-2xl dark:bg-dark-900">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold text-slate-900 dark:text-white">{{ t('playground.promptOptimizerSettings') }}</h2>
            <p class="mt-2 text-sm text-slate-400 dark:text-dark-300">
              {{ t('playground.promptOptimizerCurrent') }}：{{ imagePromptStyleLabel() }}
            </p>
          </div>
          <button
            type="button"
            class="rounded-lg p-2 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-800 dark:hover:text-white"
            @click="showPromptOptimizerModal = false"
          >
            <Icon name="x" size="md" />
          </button>
        </div>

        <div class="mt-7">
          <p class="text-sm font-semibold text-slate-400 dark:text-dark-300">{{ t('playground.promptStyle') }}</p>
          <div class="mt-3 grid grid-cols-3 gap-3">
            <button
              v-for="option in imagePromptStyleSelectOptions"
              :key="String(option.value)"
              type="button"
              class="h-12 rounded-xl border px-2 text-sm font-medium transition"
              :class="imagePromptStyle === option.value
                ? 'border-sky-500 bg-sky-50 text-sky-600 dark:bg-sky-950/40'
                : 'border-slate-200 text-slate-600 hover:bg-slate-50 dark:border-dark-700 dark:text-dark-200 dark:hover:bg-dark-800'"
              @click="imagePromptStyle = option.value as ImagePromptStyle"
            >
              {{ promptStyleOptionLabel(option) }}
            </button>
          </div>
        </div>

        <div class="mt-7">
          <label class="input-label">{{ t('playground.optimizerModel') }}</label>
          <Select
            v-model="promptOptimizerModel"
            :options="promptOptimizerModelOptions"
            :placeholder="t('playground.optimizerModel')"
            searchable
          >
            <template #selected="{ option }">
              <span v-if="option" class="playground-select-value">
                <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                  <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                </span>
                <span class="truncate">{{ option.label }}</span>
              </span>
              <span v-else>{{ t('playground.optimizerModel') }}</span>
            </template>
            <template #option="{ option, selected }">
              <div class="playground-select-option">
                <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                  <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                </span>
                <span class="min-w-0 flex-1 truncate text-left">{{ option.label }}</span>
                <Icon v-if="selected" name="check" size="sm" class="text-primary-500" />
              </div>
            </template>
          </Select>
        </div>

        <div class="mt-8 rounded-xl bg-slate-50 px-5 py-4 dark:bg-dark-950">
          <p class="text-sm font-semibold text-slate-400 dark:text-dark-300">{{ t('playground.willUse') }}</p>
          <p class="mt-2 truncate text-2xl font-bold text-slate-800 dark:text-white">{{ imagePromptStyleLabel() }}</p>
          <p class="mt-1 truncate text-xs text-slate-400 dark:text-dark-400">{{ promptOptimizerModel || t('playground.optimizerModel') }}</p>
        </div>

        <div class="mt-8 grid grid-cols-2 gap-3">
          <button type="button" class="h-12 rounded-xl bg-slate-100 text-sm font-semibold text-slate-600 transition hover:bg-slate-200 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700" @click="showPromptOptimizerModal = false">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="h-12 rounded-xl bg-blue-500 text-sm font-semibold text-white transition hover:bg-blue-600 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="optimizingPrompt || running || !draftPrompt.trim() || !selectedKey || !selectedEndpoint"
            @click="optimizeImagePrompt"
          >
            {{ optimizingPrompt ? t('playground.optimizingPrompt') : t('playground.optimizePrompt') }}
          </button>
        </div>
      </section>
    </div>

    <div v-if="imagePreview" class="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 p-4" @click.self="closeImagePreview">
      <section class="flex h-[calc(100vh-2rem)] max-h-[900px] w-full max-w-6xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl dark:bg-dark-900">
        <header class="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-slate-200 px-4 dark:border-dark-700">
          <div class="min-w-0">
            <h2 class="truncate text-sm font-semibold text-slate-900 dark:text-white">{{ t('playground.imagePreviewTitle') }}</h2>
            <p v-if="imagePreview.revisedPrompt" class="truncate text-xs text-slate-400 dark:text-dark-400">{{ imagePreview.revisedPrompt }}</p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span class="rounded-full bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-500 dark:bg-dark-800 dark:text-dark-300">
              {{ Math.round(imagePreviewZoom * 100) }}%
            </span>
            <button
              type="button"
              class="rounded-lg p-2 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-800 dark:hover:text-white"
              :title="t('common.close')"
              @click="closeImagePreview"
            >
              <Icon name="x" size="md" />
            </button>
          </div>
        </header>
        <div
          ref="imagePreviewViewport"
          class="relative min-h-0 flex-1 touch-none overflow-hidden bg-slate-950 select-none"
          :class="imagePreviewCursorClass"
          @wheel.prevent="handleImagePreviewWheel"
          @pointerdown="handleImagePreviewPointerDown"
          @pointermove="handleImagePreviewPointerMove"
          @pointerup="handleImagePreviewPointerUp"
          @pointercancel="handleImagePreviewPointerUp"
          @dblclick="resetImagePreviewView"
        >
          <img
            :src="imagePreview.url"
            :alt="imagePreview.title"
            class="pointer-events-none absolute left-1/2 top-1/2 block max-w-none object-contain will-change-transform"
            :class="imagePreviewIsDragging ? 'transition-none' : 'transition-transform duration-100'"
            :style="{
              width: `${imagePreviewBaseSize.width}px`,
              height: `${imagePreviewBaseSize.height}px`,
              transform: `translate(calc(-50% + ${imagePreviewPanX}px), calc(-50% + ${imagePreviewPanY}px)) scale(${imagePreviewZoom})`
            }"
            draggable="false"
            @load="handleImagePreviewLoad"
          />
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { keysAPI } from '@/api/keys'
import {
  cancelPlaygroundRun,
  fetchModels,
  getPlaygroundRun,
  resolvePlaygroundRequestBase,
  startPlaygroundRun,
  streamChatCompletion,
  type PlaygroundChatMessage,
  type PlaygroundImageResult,
  type PlaygroundModel,
  type PlaygroundRun,
  type PlaygroundRunRequest
} from '@/api/playground'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { formatDateOnly, formatRelativeTime, formatTime } from '@/utils/format'
import { platformIconClass } from '@/utils/platformColors'
import type { ApiKey, GroupPlatform } from '@/types'

type PlaygroundMode = 'chat' | 'image' | 'video' | 'audio'
type MessageRole = 'user' | 'assistant'
type AttachmentKind = 'image' | 'text' | 'file'
type IconName = InstanceType<typeof Icon>['$props']['name']
type ImageSizeMode = 'auto' | 'ratio' | 'custom'
type ImageResolution = '1K' | '2K' | '4K'
type ImagePromptStyle = 'auto' | 'photo' | 'illustration' | 'anime' | 'cinematic' | 'product' | 'poster' | 'watercolor' | 'pixel'
type PlaygroundRestorableRunRequest = Omit<PlaygroundRunRequest, 'apiKey'>

interface PlaygroundAttachment {
  id: string
  name: string
  type: string
  size: number
  kind: AttachmentKind
  dataUrl?: string
  text?: string
}

interface PlaygroundMessage {
  id: string
  role: MessageRole
  content: string
  createdAt: number
  model?: string
  platform?: GroupPlatform
  attachments?: PlaygroundAttachment[]
  images?: PlaygroundStoredImageResult[]
  imageConfig?: PlaygroundImageConfig
  raw?: unknown
  pending?: boolean
  progress?: string
  runId?: string
  runKeyId?: string
  runRequest?: PlaygroundRestorableRunRequest
  durationMs?: number
  error?: boolean
}

interface PlaygroundPersistedPayload {
  version: number
  savedAt: number
  activeThreadId: string
  mode: PlaygroundMode
  selectedKeyId: string
  selectedEndpointBase: string
  selectedModel: string
  selectedModelsByMode?: Partial<Record<PlaygroundMode, string>>
  systemPrompt: string
  temperature: number
  topP: number
  maxTokens: number | null
  presencePenalty: number
  frequencyPenalty: number
  imageSizeMode: ImageSizeMode
  imageResolution: ImageResolution
  imageRatio: string
  customImageWidth: number
  customImageHeight: number
  imageQuality: string
  imageCount: number
  outputFormat: string
  imagePromptStyle: ImagePromptStyle
  promptOptimizerModel: string
  showComposerConfig: boolean
  composerInputHeight: number
  threads: PlaygroundThread[]
}

interface PlaygroundThread {
  id: string
  mode: PlaygroundMode
  title: string
  messages: PlaygroundMessage[]
  createdAt: number
  updatedAt: number
  draftPrompt: string
  pendingAttachments: PlaygroundAttachment[]
  lastPrompt: string
  running?: boolean
  lastRunError?: string
  unreadCount?: number
}

interface EndpointOption {
  label: string
  value: string
  displayLabel: string
  requestBase: string
  description?: string
}

interface PlaygroundRunContext {
  mode: PlaygroundMode
  keyId: string
  apiKey: string
  endpointBase?: string
  displayEndpoint?: string
  model: string
  platform?: GroupPlatform
  temperature: number
  topP: number
  maxTokens: number | null
  presencePenalty: number
  frequencyPenalty: number
  imageSize: string
  imageCount: number
  imageQuality: string
  outputFormat: string
}

interface PlaygroundImageConfig {
  sizeMode: ImageSizeMode
  resolution: ImageResolution
  ratio: string
  customWidth: number
  customHeight: number
  size: string
  quality: string
  outputFormat: string
  count: number
}

interface PlaygroundStoredImageResult extends PlaygroundImageResult {
  storageId?: string
  mimeType?: string
  thumbnailUrl?: string
}

interface PlaygroundPersistedImage {
  id: string
  blob?: Blob
  thumbnailBlob?: Blob
  url?: string
  mimeType?: string
  revisedPrompt?: string
  savedAt: number
}

interface PlaygroundImagePersistBatch {
  records: PlaygroundPersistedImage[]
  applyThumbnails: () => void
}

interface PlaygroundRunHandle {
  controller: AbortController
  mode: PlaygroundMode
  runId?: string
}

interface PlaygroundConversationExportImage {
  id: string
  dataUrl?: string
  url?: string
  mimeType?: string
  revisedPrompt?: string
}

interface PlaygroundConversationExportPayload {
  type: 'sub2api-playground-conversations'
  version: number
  exportedAt: number
  mode: PlaygroundMode
  threads: PlaygroundThread[]
  images?: PlaygroundConversationExportImage[]
}

interface PlaygroundImagePreview {
  url: string
  title: string
  revisedPrompt?: string
}

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

marked.setOptions({
  breaks: true,
  gfm: true
})

const apiKeys = ref<ApiKey[]>([])
const selectedKeyId = ref('')
const selectedEndpointBase = ref('')
const loadingKeys = ref(false)
const loadingModels = ref(false)
const showSettings = ref(false)
const showAssistantDetails = ref(true)
const showComposerConfig = ref(true)
const composerInputHeight = ref(112)
const composerInputResizing = ref(false)
const showImageSizeModal = ref(false)
const showPromptOptimizerModal = ref(false)
const mode = ref<PlaygroundMode>('chat')
const models = ref<PlaygroundModel[]>([])
const selectedModel = ref('')
const selectedModelsByMode = ref<Partial<Record<PlaygroundMode, string>>>({})
const modelLoadError = ref('')
const systemPrompt = ref('')
const temperature = ref(0.7)
const topP = ref(1)
const maxTokens = ref<number | null>(1024)
const presencePenalty = ref(0)
const frequencyPenalty = ref(0)
const imageSizeMode = ref<ImageSizeMode>('ratio')
const imageResolution = ref<ImageResolution>('2K')
const imageRatio = ref('1:1')
const customImageWidth = ref(1024)
const customImageHeight = ref(1024)
const imageQuality = ref('auto')
const imageCount = ref(1)
const outputFormat = ref('png')
const imagePromptStyle = ref<ImagePromptStyle>('auto')
const promptOptimizerModel = ref('')
const optimizingPrompt = ref(false)
let promptOptimizeAbortController: AbortController | null = null
const threads = ref<PlaygroundThread[]>([])
const activeThreadId = ref('')
const availableChannels = ref<UserAvailableChannel[]>([])
const historySearch = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const historyImportInput = ref<HTMLInputElement | null>(null)
const messageScroller = ref<HTMLElement | null>(null)
const composerDock = ref<HTMLElement | null>(null)
const composerSpacerHeight = ref(260)
const imagePreviewViewport = ref<HTMLElement | null>(null)
const imagePreview = ref<PlaygroundImagePreview | null>(null)
const imagePreviewZoom = ref(1)
const imagePreviewPanX = ref(0)
const imagePreviewPanY = ref(0)
const imagePreviewIsDragging = ref(false)
const imagePreviewNaturalWidth = ref(0)
const imagePreviewNaturalHeight = ref(0)
const imagePreviewViewportWidth = ref(0)
const imagePreviewViewportHeight = ref(0)
let modelAbortController: AbortController | null = null
const runAbortControllers = new Map<string, PlaygroundRunHandle>()
let persistTimer: number | undefined
let restoringState = false
let persistenceReady = false
let playgroundViewMounted = false
let composerResizeObserver: ResizeObserver | null = null
let imagePreviewResizeObserver: ResizeObserver | null = null
let imagePreviewDragState: { pointerId: number; startX: number; startY: number; panX: number; panY: number } | null = null
let composerInputResizeState: { pointerId: number; startY: number; height: number } | null = null
let playgroundDBPromise: Promise<IDBDatabase> | null = null
const imageObjectURLs = new Set<string>()

const PLAYGROUND_STORAGE_VERSION = 1
const PLAYGROUND_DB_NAME = 'sub2api-playground'
const PLAYGROUND_DB_VERSION = 2
const PLAYGROUND_STATE_STORE = 'states'
const PLAYGROUND_IMAGE_STORE = 'images'
const PLAYGROUND_IMAGE_URL_PREFIX = 'playground-image://'
const PLAYGROUND_STATE_UPDATED_EVENT = 'sub2api:playground-state-updated'
const PLAYGROUND_PENDING_IMAGE_TTL_MS = 60 * 60 * 1000
const PLAYGROUND_RUN_POLL_INTERVAL_MS = 900
const PLAYGROUND_RUN_POLL_MAX_MS = 45 * 60 * 1000
const playgroundInstanceId = `playground-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
const imageResolutionEdges: Record<ImageResolution, number> = {
  '1K': 1024,
  '2K': 2048,
  '4K': 4096
}

const modeOptions = computed(() => [
  { value: 'chat' as const, label: t('playground.chatMode'), icon: 'chat' as IconName },
  { value: 'image' as const, label: t('playground.imageMode'), icon: 'sparkles' as IconName },
  { value: 'video' as const, label: t('playground.videoMode'), icon: 'play' as IconName },
  { value: 'audio' as const, label: t('playground.audioMode'), icon: 'cloud' as IconName }
])

const imageSizeOptions = computed(() => [
  { value: '1:1', label: '1:1' },
  { value: '3:2', label: '3:2' },
  { value: '2:3', label: '2:3' },
  { value: '16:9', label: '16:9' },
  { value: '9:16', label: '9:16' },
  { value: '4:3', label: '4:3' },
  { value: '3:4', label: '3:4' },
  { value: '21:9', label: '21:9' }
])

const imageSizeModeOptions = computed<Array<{ value: ImageSizeMode; label: string }>>(() => [
  { value: 'auto', label: t('playground.sizeAuto') },
  { value: 'ratio', label: t('playground.sizeByRatio') },
  { value: 'custom', label: t('playground.customWidthHeight') }
])

const imageResolutionOptions: ImageResolution[] = ['1K', '2K', '4K']

const activeKeys = computed(() => apiKeys.value.filter((key) => key.status === 'active'))
const keySelectOptions = computed<SelectOption[]>(() => activeKeys.value.map((key) => ({
  value: String(key.id),
  label: key.name,
  platform: key.platform || key.group?.platform
})))
const selectedKey = computed(() => activeKeys.value.find((key) => String(key.id) === selectedKeyId.value) || null)
const activeThread = computed(() => threads.value.find((thread) => thread.id === activeThreadId.value) || null)
const currentMessages = computed(() => activeThread.value?.messages || [])
const running = computed(() => Boolean(activeThread.value?.running))
const draftPrompt = computed({
  get: () => activeThread.value?.draftPrompt || '',
  set: (value: string) => {
    const thread = ensureActiveThread()
    thread.draftPrompt = value
    thread.updatedAt = Date.now()
  }
})
const pendingAttachments = computed({
  get: () => activeThread.value?.pendingAttachments || [],
  set: (value: PlaygroundAttachment[]) => {
    const thread = ensureActiveThread()
    thread.pendingAttachments = value
    thread.updatedAt = Date.now()
  }
})
const lastPrompt = computed({
  get: () => activeThread.value?.lastPrompt || '',
  set: (value: string) => {
    const thread = ensureActiveThread()
    thread.lastPrompt = value
    thread.updatedAt = Date.now()
  }
})
const lastRunError = computed({
  get: () => activeThread.value?.lastRunError || '',
  set: (value: string) => {
    const thread = ensureActiveThread()
    thread.lastRunError = value
    thread.updatedAt = Date.now()
  }
})
const effectiveModel = computed(() => selectedModel.value.trim())
const effectiveImageSize = computed(() => {
  if (imageSizeMode.value === 'auto') return 'auto'
  if (imageSizeMode.value === 'custom') {
    return `${clampImageDimension(customImageWidth.value)}x${clampImageDimension(customImageHeight.value)}`
  }
  return calculateImageSize(imageResolution.value, imageRatio.value)
})
const imagePreviewBaseSize = computed(() => {
  const naturalWidth = imagePreviewNaturalWidth.value || 1024
  const naturalHeight = imagePreviewNaturalHeight.value || 1024
  const viewportWidth = Math.max(imagePreviewViewportWidth.value - 32, 320)
  const viewportHeight = Math.max(imagePreviewViewportHeight.value - 32, 240)
  const fitScale = Math.min(viewportWidth / naturalWidth, viewportHeight / naturalHeight, 1)
  return {
    width: Math.max(1, Math.round(naturalWidth * fitScale)),
    height: Math.max(1, Math.round(naturalHeight * fitScale))
  }
})
const imagePreviewDisplaySize = computed(() => ({
  width: Math.max(1, Math.round(imagePreviewBaseSize.value.width * imagePreviewZoom.value)),
  height: Math.max(1, Math.round(imagePreviewBaseSize.value.height * imagePreviewZoom.value))
}))
const imagePreviewCanPan = computed(() => {
  return imagePreviewDisplaySize.value.width > imagePreviewViewportWidth.value ||
    imagePreviewDisplaySize.value.height > imagePreviewViewportHeight.value
})
const imagePreviewCursorClass = computed(() => {
  if (imagePreviewIsDragging.value) return 'cursor-grabbing'
  return imagePreviewCanPan.value ? 'cursor-grab' : 'cursor-zoom-in'
})
const publicSettings = computed(() => appStore.cachedPublicSettings)

const selectedKeyGroupIds = computed(() => {
  const key = selectedKey.value
  if (!key) return []
  const ids = Array.isArray(key.group_ids) ? [...key.group_ids] : []
  if (key.group_id && !ids.includes(key.group_id)) ids.unshift(key.group_id)
  return ids
})

const selectedKeyPlatform = computed<GroupPlatform | ''>(() => selectedKey.value?.platform || selectedKey.value?.group?.platform || '')

const userDisplayName = computed(() => {
  const user = authStore.user
  return user?.email || user?.username || t('playground.you')
})
const userAvatarUrl = computed(() => authStore.user?.avatar_url?.trim() || '')
const userAvatarInitial = computed(() => {
  const name = authStore.user?.username?.trim() || authStore.user?.email?.split('@')[0]?.trim() || ''
  return name.slice(0, 2).toUpperCase() || 'U'
})

const endpointOptions = computed<EndpointOption[]>(() => {
  const items: EndpointOption[] = []
  const seen = new Set<string>()
  const pushEndpoint = (label: string, value: string, description?: string) => {
    const normalized = normalizeEndpointBase(value || window.location.origin)
    if (!normalized || seen.has(normalized)) return
    seen.add(normalized)
    const requestBase = resolvePlaygroundRequestBase(normalized)
    items.push({
      label,
      value: normalized,
      displayLabel: `${label} - ${normalized}`,
      requestBase,
      description
    })
  }

  pushEndpoint(t('playground.defaultEndpoint'), publicSettings.value?.api_base_url || window.location.origin)
  for (const endpoint of publicSettings.value?.custom_endpoints || []) {
    pushEndpoint(endpoint.name || endpoint.endpoint, endpoint.endpoint, endpoint.description)
  }

  return items
})

const selectedEndpoint = computed(() => endpointOptions.value.find((item) => item.value === selectedEndpointBase.value) || endpointOptions.value[0] || null)
const endpointSelectOptions = computed<SelectOption[]>(() => endpointOptions.value.map((endpoint) => ({
  value: endpoint.value,
  label: endpoint.displayLabel,
  description: endpoint.description || endpoint.requestBase
})))
const selectedEndpointLabel = computed(() => {
  if (!selectedEndpoint.value) return ''
  return `${t('playground.endpoint')}: ${selectedEndpoint.value.value} (${t('playground.requestPath')}: ${selectedEndpoint.value.requestBase})`
})
const endpointSelectWidth = computed(() => {
  const label = selectedEndpoint.value?.displayLabel || t('playground.selectEndpoint')
  const length = Array.from(label).length
  return `${Math.min(Math.max(length + 3, 12), 42)}ch`
})

const playgroundNotice = computed(() => {
  if (lastRunError.value) {
    return { type: 'error' as const, message: lastRunError.value }
  }
  if (modelLoadError.value) {
    return { type: 'warning' as const, message: modelLoadError.value }
  }
  return null
})

const fallbackImageModels = computed<PlaygroundModel[]>(() => [
  { id: 'gpt-image-2', label: 'gpt-image-2' },
  { id: 'gpt-image-1.5', label: 'gpt-image-1.5' },
  { id: 'gpt-image-1', label: 'gpt-image-1' }
])

const fallbackChatModels = computed<PlaygroundModel[]>(() => {
  if (selectedKeyPlatform.value === 'openai') {
    return [{ id: 'gpt-5.5', label: 'gpt-5.5' }]
  }
  if (isClaudePlatform(selectedKeyPlatform.value)) {
    return [{ id: 'claude-opus-4-8', label: 'claude-opus-4-8' }]
  }
  return []
})

const channelModels = computed<PlaygroundModel[]>(() => {
  const platform = selectedKeyPlatform.value
  const groupIds = selectedKeyGroupIds.value
  const seen = new Set<string>()
  const result: PlaygroundModel[] = []

  for (const channel of availableChannels.value) {
    for (const section of channel.platforms || []) {
      const sectionPlatform = section.platform as GroupPlatform
      const platformMatches = !platform || sectionPlatform === platform
      const groupMatches =
        groupIds.length === 0 ||
        section.groups.some((group) => groupIds.includes(group.id))
      if (!platformMatches || !groupMatches) continue

      for (const model of section.supported_models || []) {
        if (!model.name || seen.has(model.name)) continue
        seen.add(model.name)
        result.push({
          id: model.name,
          label: model.name,
          owned_by: model.platform || section.platform || channel.name
        })
      }
    }
  }

  return result
})
const mergedModels = computed<PlaygroundModel[]>(() => {
  const seen = new Set<string>()
  const result: PlaygroundModel[] = []
  for (const model of [...models.value, ...channelModels.value]) {
    if (!model.id || seen.has(model.id)) continue
    seen.add(model.id)
    result.push(model)
  }
  return result
})
const chatModels = computed(() => {
  const seen = new Set<string>()
  const result: PlaygroundModel[] = []
  for (const model of [...fallbackChatModels.value, ...mergedModels.value.filter((item) => !isImageModel(item.id))]) {
    if (!model.id || seen.has(model.id)) continue
    seen.add(model.id)
    result.push(model)
  }
  return result
})
const imageModels = computed(() => {
  const seen = new Set<string>()
  const result: PlaygroundModel[] = []
  for (const model of [...fallbackImageModels.value, ...mergedModels.value.filter((item) => isImageModel(item.id))]) {
    if (!model.id || seen.has(model.id)) continue
    seen.add(model.id)
    result.push(model)
  }
  return result
})
const visibleModels = computed(() => mode.value === 'image' ? imageModels.value : chatModels.value)
const modelSelectOptions = computed<SelectOption[]>(() => visibleModels.value.map((model) => ({
  value: model.id,
  label: model.label || model.id,
  description: model.owned_by,
  platform: platformForModel(model.id, model.owned_by)
})))

const imageQualitySelectOptions = computed<SelectOption[]>(() => [
  { value: 'auto', label: `${t('playground.quality')}：${t('playground.qualityAuto')}` },
  { value: 'low', label: `${t('playground.quality')}：${t('playground.qualityLow')}` },
  { value: 'medium', label: `${t('playground.quality')}：${t('playground.qualityMedium')}` },
  { value: 'high', label: `${t('playground.quality')}：${t('playground.qualityHigh')}` }
])

const imageQualitySettingOptions = computed<SelectOption[]>(() => [
  { value: 'auto', label: t('playground.qualityAuto') },
  { value: 'low', label: t('playground.qualityLow') },
  { value: 'medium', label: t('playground.qualityMedium') },
  { value: 'high', label: t('playground.qualityHigh') }
])

const outputFormatSelectOptions = computed<SelectOption[]>(() => [
  { value: 'png', label: `${t('playground.outputFormat')}：PNG` },
  { value: 'webp', label: `${t('playground.outputFormat')}：WEBP` },
  { value: 'jpeg', label: `${t('playground.outputFormat')}：JPEG` }
])

const imagePromptStyleSelectOptions = computed<SelectOption[]>(() => [
  { value: 'auto', label: `${t('playground.promptStyle')}：${t('playground.promptStyleAuto')}` },
  { value: 'photo', label: `${t('playground.promptStyle')}：${t('playground.promptStylePhoto')}` },
  { value: 'illustration', label: `${t('playground.promptStyle')}：${t('playground.promptStyleIllustration')}` },
  { value: 'anime', label: `${t('playground.promptStyle')}：${t('playground.promptStyleAnime')}` },
  { value: 'cinematic', label: `${t('playground.promptStyle')}：${t('playground.promptStyleCinematic')}` },
  { value: 'product', label: `${t('playground.promptStyle')}：${t('playground.promptStyleProduct')}` },
  { value: 'poster', label: `${t('playground.promptStyle')}：${t('playground.promptStylePoster')}` },
  { value: 'watercolor', label: `${t('playground.promptStyle')}：${t('playground.promptStyleWatercolor')}` },
  { value: 'pixel', label: `${t('playground.promptStyle')}：${t('playground.promptStylePixel')}` }
])

const promptOptimizerModelOptions = computed<SelectOption[]>(() => chatModels.value.map((model) => ({
  value: model.id,
  label: model.label || model.id,
  description: model.owned_by,
  platform: platformForModel(model.id, model.owned_by)
})))

const modeThreads = computed(() => threads.value.filter((thread) => thread.mode === mode.value))

const filteredThreads = computed(() => {
  const keyword = historySearch.value.trim().toLowerCase()
  if (!keyword) return modeThreads.value
  return modeThreads.value.filter((thread) => {
    return thread.title.toLowerCase().includes(keyword) ||
      thread.messages.some((message) => message.content.toLowerCase().includes(keyword))
  })
})

const composerPlaceholder = computed(() => {
  if (mode.value === 'image') return t('playground.imagePromptPlaceholder')
  if (mode.value === 'video') return t('playground.videoPromptPlaceholder')
  if (mode.value === 'audio') return t('playground.audioPromptPlaceholder')
  return t('playground.userPromptPlaceholder')
})

const modeHint = computed(() => {
  if (mode.value === 'image') return t('playground.imageInputHint')
  if (mode.value === 'video') return t('playground.videoInputHint')
  if (mode.value === 'audio') return t('playground.audioInputHint')
  return t('playground.chatInputHint')
})

function uid(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function dataURLToBlob(dataUrl: string): Blob | null {
  const match = dataUrl.match(/^data:([^;,]+)?(;base64)?,(.*)$/)
  if (!match) return null
  const mimeType = match[1] || 'application/octet-stream'
  const isBase64 = Boolean(match[2])
  const data = match[3] || ''
  try {
    const binary = isBase64 ? atob(data) : decodeURIComponent(data)
    const bytes = new Uint8Array(binary.length)
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index)
    }
    return new Blob([bytes], { type: mimeType })
  } catch {
    return null
  }
}

function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error || new Error('Failed to read image data'))
    reader.readAsDataURL(blob)
  })
}

function storedImagePlaceholder(storageId: string): string {
  return `${PLAYGROUND_IMAGE_URL_PREFIX}${storageId}`
}

function storedImageIdFromURL(url: string): string {
  return url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX)
    ? url.slice(PLAYGROUND_IMAGE_URL_PREFIX.length)
    : ''
}

function ensureImageStorageIds() {
  for (const thread of threads.value) {
    for (const message of thread.messages) {
      message.images?.forEach((image, index) => {
        if (!image.storageId) {
          image.storageId = uid(`image-${message.id}-${index}`)
        }
      })
    }
  }
}

function createTrackedObjectURL(blob: Blob): string {
  const url = URL.createObjectURL(blob)
  imageObjectURLs.add(url)
  return url
}

function revokeTrackedObjectURL(url?: string) {
  if (!url?.startsWith('blob:')) return
  URL.revokeObjectURL(url)
  imageObjectURLs.delete(url)
}

function revokeImageObjectURLs(images?: PlaygroundStoredImageResult[]) {
  for (const image of images || []) {
    revokeTrackedObjectURL(image.url)
  }
}

function revokeAllImageObjectURLs() {
  for (const url of imageObjectURLs) {
    URL.revokeObjectURL(url)
  }
  imageObjectURLs.clear()
}

async function createImageThumbnailBlob(blob: Blob, maxEdge = 640): Promise<Blob> {
  const image = await new Promise<HTMLImageElement>((resolve, reject) => {
    const objectUrl = URL.createObjectURL(blob)
    const element = new Image()
    element.onload = () => {
      URL.revokeObjectURL(objectUrl)
      resolve(element)
    }
    element.onerror = () => {
      URL.revokeObjectURL(objectUrl)
      reject(new Error('Failed to load image for thumbnail'))
    }
    element.src = objectUrl
  })
  const ratio = image.naturalWidth > 0 && image.naturalHeight > 0
    ? Math.min(1, maxEdge / Math.max(image.naturalWidth, image.naturalHeight))
    : 1
  const width = Math.max(1, Math.round((image.naturalWidth || maxEdge) * ratio))
  const height = Math.max(1, Math.round((image.naturalHeight || maxEdge) * ratio))
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const context = canvas.getContext('2d')
  if (!context) return blob
  context.drawImage(image, 0, 0, width, height)
  return await new Promise((resolve) => {
    canvas.toBlob((thumbnail) => resolve(thumbnail || blob), 'image/webp', 0.82)
  })
}

function normalizeEndpointBase(value: string): string {
  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed) return ''
  return trimmed
}

function clampImageDimension(value: number): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return 1024
  return Math.min(Math.max(Math.round(parsed), 256), 4096)
}

function clampComposerInputHeight(value: number): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return 112
  return Math.min(Math.max(Math.round(parsed), 96), 360)
}

function calculateImageSize(resolution: ImageResolution, ratio: string): string {
  const edge = imageResolutionEdges[resolution] || imageResolutionEdges['2K']
  const [rawW, rawH] = ratio.split(':').map((item) => Number(item))
  if (!rawW || !rawH || ratio === '1:1') {
    return `${edge}x${edge}`
  }
  if (rawW >= rawH) {
    const height = Math.max(256, Math.round(edge * rawH / rawW))
    return `${edge}x${height}`
  }
  const width = Math.max(256, Math.round(edge * rawW / rawH))
  return `${width}x${edge}`
}

function isClaudePlatform(platform: string): boolean {
  return platform === 'anthropic' || platform === 'antigravity'
}

function isImagePromptStyle(value: unknown): value is ImagePromptStyle {
  return value === 'auto' ||
    value === 'photo' ||
    value === 'illustration' ||
    value === 'anime' ||
    value === 'cinematic' ||
    value === 'product' ||
    value === 'poster' ||
    value === 'watercolor' ||
    value === 'pixel'
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function isPlaygroundMode(value: unknown): value is PlaygroundMode {
  return value === 'chat' || value === 'image' || value === 'video' || value === 'audio'
}

function isGroupPlatform(value: unknown): value is GroupPlatform {
  return value === 'anthropic' || value === 'openai' || value === 'gemini' || value === 'antigravity'
}

function optionPlatform(option: SelectOption | Record<string, unknown> | null): GroupPlatform | undefined {
  if (!option || typeof option !== 'object') return undefined
  const platform = option.platform
  return isGroupPlatform(platform) ? platform : undefined
}

function platformForModel(modelID: string, owner?: string): GroupPlatform | undefined {
  const source = `${modelID} ${owner || ''}`.toLowerCase()
  if (source.includes('claude') || source.includes('anthropic')) return 'anthropic'
  if (source.includes('gemini') || source.includes('google')) return 'gemini'
  if (source.includes('antigravity')) return 'antigravity'
  if (/(\bgpt\b|gpt-|^o\d|dall-e|openai|image)/i.test(source)) return 'openai'
  return isGroupPlatform(selectedKeyPlatform.value) ? selectedKeyPlatform.value : undefined
}

function messagePlatform(message: PlaygroundMessage): GroupPlatform | undefined {
  if (message.platform) return message.platform
  return platformForModel(message.model || '')
}

function isImageModel(model: string): boolean {
  return /(^|[-_])(image|dall-e|flux|sd|midjourney)/i.test(model) || /^gpt-image-/i.test(model)
}

function preferredModelForMode(targetMode: PlaygroundMode = mode.value): string {
  if (targetMode === 'image') return 'gpt-image-2'
  if (selectedKeyPlatform.value === 'openai') return 'gpt-5.5'
  if (isClaudePlatform(selectedKeyPlatform.value)) return 'claude-opus-4-8'
  return visibleModels.value[0]?.id || ''
}

function modelAvailable(modelID: string): boolean {
  if (!modelID) return false
  return visibleModels.value.some((model) => model.id === modelID)
}

function createEmptyThread(nextMode: PlaygroundMode): PlaygroundThread {
  return {
    id: uid('thread'),
    mode: nextMode,
    title: t('playground.untitledConversation'),
    messages: [],
    createdAt: Date.now(),
    updatedAt: Date.now(),
    draftPrompt: '',
    pendingAttachments: [],
    lastPrompt: '',
    unreadCount: 0
  }
}

function normalizeRestoredMessageContent(message: PlaygroundMessage): string {
  const content = typeof message.content === 'string' ? message.content : ''
  if (!message.pending) return content
  if (message.runId) return content
  if (message.images?.length) return content || t('playground.imageGenerated')
  if (isRecoverablePendingImageMessage(message)) {
    return content
  }
  if (!content || content === t('playground.generatingImages')) return t('playground.requestInterrupted')
  return content
}

function isRecoverablePendingImageMessage(message: Partial<PlaygroundMessage>): boolean {
  if (message.pending && message.runId) return true
  return Boolean(
    message.pending &&
    message.imageConfig &&
    typeof message.createdAt === 'number' &&
    Date.now() - message.createdAt < PLAYGROUND_PENDING_IMAGE_TTL_MS
  )
}

function normalizeThread(thread: Partial<PlaygroundThread>): PlaygroundThread {
  const normalized = createEmptyThread(isPlaygroundMode(thread.mode) ? thread.mode : 'chat')
  normalized.id = typeof thread.id === 'string' && thread.id ? thread.id : normalized.id
  normalized.title = typeof thread.title === 'string' && thread.title ? thread.title : normalized.title
  normalized.messages = Array.isArray(thread.messages)
    ? thread.messages
      .filter((message): message is PlaygroundMessage => Boolean(message && typeof message === 'object'))
      .map((message) => {
        const recoverablePendingImage = isRecoverablePendingImageMessage(message)
        const recoverablePendingRun = Boolean(message.pending && message.runId)
        return {
          ...message,
          pending: recoverablePendingRun || recoverablePendingImage,
          platform: isGroupPlatform(message.platform) ? message.platform : platformForModel(message.model || ''),
          progress: recoverablePendingRun || recoverablePendingImage
            ? (message.progress || (message.imageConfig ? t('playground.generatingImages') : t('playground.streaming')))
            : '',
          content: normalizeRestoredMessageContent(message)
        }
      })
    : []
  normalized.createdAt = typeof thread.createdAt === 'number' ? thread.createdAt : normalized.createdAt
  normalized.updatedAt = typeof thread.updatedAt === 'number' ? thread.updatedAt : normalized.updatedAt
  normalized.draftPrompt = typeof thread.draftPrompt === 'string' ? thread.draftPrompt : ''
  normalized.pendingAttachments = Array.isArray(thread.pendingAttachments) ? thread.pendingAttachments : []
  normalized.lastPrompt = typeof thread.lastPrompt === 'string' ? thread.lastPrompt : ''
  normalized.unreadCount = typeof thread.unreadCount === 'number' ? thread.unreadCount : 0
  normalized.running = normalized.messages.some((message) => Boolean(message.pending && message.runId))
  normalized.lastRunError = typeof thread.lastRunError === 'string' ? thread.lastRunError : ''
  return normalized
}

function cloneThreadForTransfer(thread: PlaygroundThread): PlaygroundThread {
  return JSON.parse(JSON.stringify(thread)) as PlaygroundThread
}

async function rewriteImportedThreadIds(
  thread: PlaygroundThread,
  exportedImages: Map<string, PlaygroundConversationExportImage>,
  imageRecords: PlaygroundPersistedImage[]
): Promise<PlaygroundThread> {
  const normalized = normalizeThread(thread)
  normalized.id = uid('thread')
  normalized.createdAt = typeof thread.createdAt === 'number' ? thread.createdAt : Date.now()
  normalized.updatedAt = Date.now()
  normalized.running = false
  normalized.unreadCount = 0
  normalized.lastRunError = ''
  normalized.pendingAttachments = []
  normalized.messages = await Promise.all(normalized.messages.map(async (message, messageIndex) => {
    const nextMessage: PlaygroundMessage = {
      ...message,
      id: uid(`msg-${messageIndex}`),
      pending: false,
      progress: '',
      raw: undefined
    }
    const nextImages: PlaygroundStoredImageResult[] = []
    for (const [imageIndex, image] of (nextMessage.images || []).entries()) {
      const oldStorageId = image.storageId || storedImageIdFromURL(image.url)
      const exportedImage = oldStorageId ? exportedImages.get(oldStorageId) : undefined
      const dataUrl = exportedImage?.dataUrl || (image.url?.startsWith('data:') ? image.url : '')
      const remoteUrl = exportedImage?.url ||
        (image.url && !image.url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX) && !image.url.startsWith('blob:') && !image.url.startsWith('data:')
          ? image.url
          : '')
      const storageId = dataUrl || remoteUrl ? uid(`image-${nextMessage.id}-${imageIndex}`) : ''
      if (dataUrl && storageId) {
        const blob = dataURLToBlob(dataUrl)
        if (!blob) continue
        const thumbnailBlob = await createImageThumbnailBlob(blob).catch(() => blob)
        const thumbnailUrl = createTrackedObjectURL(thumbnailBlob)
        imageRecords.push({
          id: storageId,
          blob,
          thumbnailBlob,
          mimeType: exportedImage?.mimeType || image.mimeType || blob.type,
          revisedPrompt: image.revisedPrompt || exportedImage?.revisedPrompt,
          savedAt: Date.now()
        })
        nextImages.push({
          ...image,
          storageId,
          mimeType: exportedImage?.mimeType || image.mimeType || blob.type,
          revisedPrompt: image.revisedPrompt || exportedImage?.revisedPrompt,
          url: thumbnailUrl,
          thumbnailUrl
        })
      } else if (remoteUrl && storageId) {
        imageRecords.push({
          id: storageId,
          url: remoteUrl,
          mimeType: exportedImage?.mimeType || image.mimeType,
          revisedPrompt: image.revisedPrompt || exportedImage?.revisedPrompt,
          savedAt: Date.now()
        })
        nextImages.push({
          ...image,
          storageId,
          mimeType: exportedImage?.mimeType || image.mimeType,
          revisedPrompt: image.revisedPrompt || exportedImage?.revisedPrompt,
          url: remoteUrl,
          thumbnailUrl: undefined
        })
      }
    }
    nextMessage.images = nextImages
    return nextMessage
  }))
  return normalized
}

function storageKey(): string {
  return `sub2api:playground:v${PLAYGROUND_STORAGE_VERSION}:${authStore.user?.id || authStore.user?.email || 'guest'}`
}

function openPlaygroundDB(): Promise<IDBDatabase> {
  if (!playgroundDBPromise) {
    playgroundDBPromise = new Promise((resolve, reject) => {
      const request = indexedDB.open(PLAYGROUND_DB_NAME, PLAYGROUND_DB_VERSION)
      request.onupgradeneeded = () => {
        const db = request.result
        if (!db.objectStoreNames.contains(PLAYGROUND_STATE_STORE)) {
          db.createObjectStore(PLAYGROUND_STATE_STORE)
        }
        if (!db.objectStoreNames.contains(PLAYGROUND_IMAGE_STORE)) {
          db.createObjectStore(PLAYGROUND_IMAGE_STORE)
        }
      }
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error || new Error('Failed to open playground database'))
    })
  }
  return playgroundDBPromise
}

async function savePlaygroundImagesToDB(images: PlaygroundPersistedImage[]) {
  if (images.length === 0) return
  const db = await openPlaygroundDB()
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction(PLAYGROUND_IMAGE_STORE, 'readwrite')
    const store = transaction.objectStore(PLAYGROUND_IMAGE_STORE)
    for (const image of images) {
      store.put(image, image.id)
    }
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error || new Error('Failed to write playground images'))
  })
}

async function loadPlaygroundImageFromDB(id: string): Promise<PlaygroundPersistedImage | null> {
  if (!id) return null
  const db = await openPlaygroundDB()
  return await new Promise((resolve, reject) => {
    const transaction = db.transaction(PLAYGROUND_IMAGE_STORE, 'readonly')
    const request = transaction.objectStore(PLAYGROUND_IMAGE_STORE).get(id)
    request.onsuccess = () => resolve((request.result as PlaygroundPersistedImage | undefined) || null)
    request.onerror = () => reject(request.error || new Error('Failed to read playground image'))
  })
}

async function collectPersistedImagesFromThreads(): Promise<PlaygroundImagePersistBatch> {
  const records: PlaygroundPersistedImage[] = []
  const replacements: Array<{ image: PlaygroundStoredImageResult; thumbnailBlob: Blob }> = []
  for (const thread of threads.value) {
    for (const message of thread.messages) {
      for (const image of message.images || []) {
        if (!image.storageId) continue
        if (image.url.startsWith('data:')) {
          const blob = dataURLToBlob(image.url)
          if (!blob) continue
          const thumbnailBlob = await createImageThumbnailBlob(blob).catch(() => blob)
          image.mimeType = image.mimeType || blob.type
          replacements.push({ image, thumbnailBlob })
          records.push({
            id: image.storageId,
            blob,
            thumbnailBlob,
            mimeType: image.mimeType || blob.type,
            revisedPrompt: image.revisedPrompt,
            savedAt: Date.now()
          })
        } else if (image.url && !image.url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX) && !image.url.startsWith('blob:')) {
          records.push({
            id: image.storageId,
            url: image.url,
            mimeType: image.mimeType,
            revisedPrompt: image.revisedPrompt,
            savedAt: Date.now()
          })
        }
      }
    }
  }
  return {
    records,
    applyThumbnails() {
      for (const replacement of replacements) {
        replacement.image.url = createTrackedObjectURL(replacement.thumbnailBlob)
        replacement.image.thumbnailUrl = replacement.image.url
      }
    }
  }
}

async function persistImageMessageAssets(message: PlaygroundMessage): Promise<PlaygroundPersistedImage[]> {
  const records: PlaygroundPersistedImage[] = []
  for (const [index, image] of (message.images || []).entries()) {
    if (!image.storageId) {
      image.storageId = uid(`image-${message.id}-${index}`)
    }
    if (image.url.startsWith('data:')) {
      const blob = dataURLToBlob(image.url)
      if (!blob) continue
      const thumbnailBlob = await createImageThumbnailBlob(blob).catch(() => blob)
      image.mimeType = image.mimeType || blob.type
      records.push({
        id: image.storageId,
        blob,
        thumbnailBlob,
        mimeType: image.mimeType || blob.type,
        revisedPrompt: image.revisedPrompt,
        savedAt: Date.now()
      })
      if (playgroundViewMounted) {
        image.url = createTrackedObjectURL(thumbnailBlob)
        image.thumbnailUrl = image.url
      } else {
        image.url = storedImagePlaceholder(image.storageId)
        image.thumbnailUrl = undefined
      }
    } else if (image.url && !image.url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX) && !image.url.startsWith('blob:')) {
      records.push({
        id: image.storageId,
        url: image.url,
        mimeType: image.mimeType,
        revisedPrompt: image.revisedPrompt,
        savedAt: Date.now()
      })
      image.thumbnailUrl = undefined
    }
  }
  return records
}

function serializeMessageForPersistence(message: PlaygroundMessage): PlaygroundMessage {
  const recoverablePendingRun = Boolean(message.pending && message.runId)
  const recoverablePendingImage = isRecoverablePendingImageMessage(message)
  return {
    ...message,
    images: message.images?.map((image) => ({
      ...image,
      url: image.storageId ? storedImagePlaceholder(image.storageId) : image.url,
      thumbnailUrl: undefined
    })) || [],
    pending: recoverablePendingRun || recoverablePendingImage,
    raw: undefined,
    progress: recoverablePendingRun || recoverablePendingImage
      ? (message.progress || (message.imageConfig ? t('playground.generatingImages') : t('playground.streaming')))
      : '',
    content: normalizeRestoredMessageContent(message)
  }
}

async function hydratePersistedImages() {
  const restoreJobs: Array<Promise<void>> = []
  for (const thread of threads.value) {
    for (const message of thread.messages) {
      if (!message.images?.length) continue
      message.images = message.images.filter((image) => image.url || image.storageId)
      for (const image of message.images) {
        const storageId = image.storageId || storedImageIdFromURL(image.url)
        if (!storageId) continue
        image.storageId = storageId
        if (!image.url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX)) continue
        restoreJobs.push(loadPlaygroundImageFromDB(storageId)
          .then(async (persisted) => {
            if (!persisted) return
            if (persisted.thumbnailBlob) {
              image.url = createTrackedObjectURL(persisted.thumbnailBlob)
              image.thumbnailUrl = image.url
            } else if (persisted.blob) {
              const thumbnailBlob = await createImageThumbnailBlob(persisted.blob).catch(() => persisted.blob as Blob)
              image.url = createTrackedObjectURL(thumbnailBlob)
              image.thumbnailUrl = image.url
            } else if (persisted.url) {
              image.url = persisted.url
            }
            image.mimeType = image.mimeType || persisted.mimeType
            image.revisedPrompt = image.revisedPrompt || persisted.revisedPrompt
          })
          .catch((error) => {
            console.warn('Failed to restore playground image:', error)
          }))
      }
    }
  }
  await Promise.all(restoreJobs)
  for (const thread of threads.value) {
    for (const message of thread.messages) {
      const originalLength = message.images?.length || 0
      message.images = message.images?.filter((image) => image.url && !image.url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX)) || []
      if (originalLength > 0 && message.images.length === 0 && message.content === t('playground.imageGenerated')) {
        message.content = t('playground.imageCacheMissing')
      }
    }
  }
}

async function savePlaygroundStateToDB(payload: PlaygroundPersistedPayload) {
  const db = await openPlaygroundDB()
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction(PLAYGROUND_STATE_STORE, 'readwrite')
    transaction.objectStore(PLAYGROUND_STATE_STORE).put(payload, storageKey())
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error || new Error('Failed to write playground state'))
  })
}

async function loadPlaygroundStateFromDB(): Promise<PlaygroundPersistedPayload | null> {
  const db = await openPlaygroundDB()
  return await new Promise((resolve, reject) => {
    const transaction = db.transaction(PLAYGROUND_STATE_STORE, 'readonly')
    const request = transaction.objectStore(PLAYGROUND_STATE_STORE).get(storageKey())
    request.onsuccess = () => resolve((request.result as PlaygroundPersistedPayload | undefined) || null)
    request.onerror = () => reject(request.error || new Error('Failed to read playground state'))
  })
}

function buildPlaygroundPayload(): PlaygroundPersistedPayload {
  return {
    version: PLAYGROUND_STORAGE_VERSION,
    savedAt: Date.now(),
    activeThreadId: activeThreadId.value,
    mode: mode.value,
    selectedKeyId: selectedKeyId.value,
    selectedEndpointBase: selectedEndpointBase.value,
    selectedModel: selectedModel.value,
    selectedModelsByMode: selectedModelsByMode.value,
    systemPrompt: systemPrompt.value,
    temperature: temperature.value,
    topP: topP.value,
    maxTokens: maxTokens.value,
    presencePenalty: presencePenalty.value,
    frequencyPenalty: frequencyPenalty.value,
    imageSizeMode: imageSizeMode.value,
    imageResolution: imageResolution.value,
    imageRatio: imageRatio.value,
    customImageWidth: customImageWidth.value,
    customImageHeight: customImageHeight.value,
    imageQuality: imageQuality.value,
    imageCount: imageCount.value,
    outputFormat: outputFormat.value,
    imagePromptStyle: imagePromptStyle.value,
    promptOptimizerModel: promptOptimizerModel.value,
    showComposerConfig: showComposerConfig.value,
    composerInputHeight: composerInputHeight.value,
    threads: threads.value.map((thread) => ({
      ...thread,
      running: false,
      messages: thread.messages.map((message) => ({
        ...message,
        images: message.images?.map((image) => ({
          ...image,
          url: image.storageId ? storedImagePlaceholder(image.storageId) : image.url
        })) || [],
        pending: Boolean(message.pending && message.runId) || isRecoverablePendingImageMessage(message),
        raw: undefined,
        progress: (Boolean(message.pending && message.runId) || isRecoverablePendingImageMessage(message))
          ? (message.progress || (message.imageConfig ? t('playground.generatingImages') : t('playground.streaming')))
          : '',
        content: normalizeRestoredMessageContent(message)
      }))
    }))
  }
}

function buildLocalStoragePayload(payload: PlaygroundPersistedPayload): PlaygroundPersistedPayload {
  return {
    ...payload,
    threads: payload.threads.map((thread) => ({
      ...thread,
      messages: thread.messages.map((message) => ({
        ...message,
        images: message.images?.map((image) => ({
          ...image,
          url: image.storageId ? storedImagePlaceholder(image.storageId) : image.url
        })).filter((image) => image.url || image.storageId) || []
      }))
    }))
  }
}

function applyPlaygroundPayload(payload: Record<string, unknown>) {
  if (payload.version !== PLAYGROUND_STORAGE_VERSION) return

  const savedThreads = Array.isArray(payload.threads)
    ? payload.threads.map((thread) => normalizeThread(thread as Partial<PlaygroundThread>))
    : []
  if (savedThreads.length > 0) {
    threads.value = savedThreads
    const savedActiveId = typeof payload.activeThreadId === 'string' ? payload.activeThreadId : ''
    activeThreadId.value = savedThreads.some((thread) => thread.id === savedActiveId)
      ? savedActiveId
      : savedThreads[0].id
    mode.value = activeThread.value?.mode || (typeof payload.mode === 'string' ? payload.mode as PlaygroundMode : 'chat')
  }

  selectedKeyId.value = typeof payload.selectedKeyId === 'string' ? payload.selectedKeyId : selectedKeyId.value
  selectedEndpointBase.value = typeof payload.selectedEndpointBase === 'string' ? payload.selectedEndpointBase : selectedEndpointBase.value
  selectedModel.value = typeof payload.selectedModel === 'string' ? payload.selectedModel : selectedModel.value
  if (payload.selectedModelsByMode && typeof payload.selectedModelsByMode === 'object') {
    const savedModels = payload.selectedModelsByMode as Partial<Record<PlaygroundMode, string>>
    selectedModelsByMode.value = {
      chat: typeof savedModels.chat === 'string' ? savedModels.chat : undefined,
      image: typeof savedModels.image === 'string' ? savedModels.image : undefined,
      video: typeof savedModels.video === 'string' ? savedModels.video : undefined,
      audio: typeof savedModels.audio === 'string' ? savedModels.audio : undefined
    }
  }
  systemPrompt.value = typeof payload.systemPrompt === 'string' ? payload.systemPrompt : systemPrompt.value
  temperature.value = typeof payload.temperature === 'number' ? payload.temperature : temperature.value
  topP.value = typeof payload.topP === 'number' ? payload.topP : topP.value
  maxTokens.value = typeof payload.maxTokens === 'number' ? payload.maxTokens : maxTokens.value
  presencePenalty.value = typeof payload.presencePenalty === 'number' ? payload.presencePenalty : presencePenalty.value
  frequencyPenalty.value = typeof payload.frequencyPenalty === 'number' ? payload.frequencyPenalty : frequencyPenalty.value
  imageSizeMode.value = ['auto', 'ratio', 'custom'].includes(String(payload.imageSizeMode)) ? payload.imageSizeMode as ImageSizeMode : imageSizeMode.value
  imageResolution.value = ['1K', '2K', '4K'].includes(String(payload.imageResolution)) ? payload.imageResolution as ImageResolution : imageResolution.value
  imageRatio.value = typeof payload.imageRatio === 'string' ? payload.imageRatio : imageRatio.value
  customImageWidth.value = typeof payload.customImageWidth === 'number' ? payload.customImageWidth : customImageWidth.value
  customImageHeight.value = typeof payload.customImageHeight === 'number' ? payload.customImageHeight : customImageHeight.value
  imageQuality.value = typeof payload.imageQuality === 'string' ? payload.imageQuality : imageQuality.value
  imageCount.value = typeof payload.imageCount === 'number' ? payload.imageCount : imageCount.value
  outputFormat.value = typeof payload.outputFormat === 'string' ? payload.outputFormat : outputFormat.value
  imagePromptStyle.value = isImagePromptStyle(payload.imagePromptStyle) ? payload.imagePromptStyle : imagePromptStyle.value
  promptOptimizerModel.value = typeof payload.promptOptimizerModel === 'string' ? payload.promptOptimizerModel : promptOptimizerModel.value
  showComposerConfig.value = typeof payload.showComposerConfig === 'boolean' ? payload.showComposerConfig : showComposerConfig.value
  composerInputHeight.value = typeof payload.composerInputHeight === 'number'
    ? clampComposerInputHeight(payload.composerInputHeight)
    : composerInputHeight.value
}

async function writePlaygroundStateNow() {
  if (restoringState) return
  if (persistTimer !== undefined) {
    window.clearTimeout(persistTimer)
    persistTimer = undefined
  }
  ensureImageStorageIds()
  const imageBatch = await collectPersistedImagesFromThreads()
  const payload = buildPlaygroundPayload()
  try {
    await savePlaygroundImagesToDB(imageBatch.records)
    imageBatch.applyThumbnails()
    await savePlaygroundStateToDB(payload)
    localStorage.setItem(storageKey(), JSON.stringify(buildLocalStoragePayload(payload)))
    window.dispatchEvent(new CustomEvent(PLAYGROUND_STATE_UPDATED_EVENT, { detail: { key: storageKey(), source: playgroundInstanceId } }))
  } catch (error) {
    console.warn('Failed to persist playground state:', error)
    try {
      localStorage.setItem(storageKey(), JSON.stringify(buildLocalStoragePayload(payload)))
    } catch (fallbackError) {
      console.warn('Failed to persist playground fallback state:', fallbackError)
    }
  }
}

async function persistCompletedImageMessage(thread: PlaygroundThread, assistantMessage: PlaygroundMessage) {
  try {
    const imageRecords = await persistImageMessageAssets(assistantMessage)
    const currentPayload = await loadPlaygroundStateFromDB().catch(() => null)
    const basePayload = currentPayload || buildPlaygroundPayload()
    const persistedMessage = serializeMessageForPersistence(assistantMessage)
    let foundThread = false
    const nextPayload: PlaygroundPersistedPayload = {
      ...basePayload,
      savedAt: Date.now(),
      threads: basePayload.threads.map((savedThread) => {
        if (savedThread.id !== thread.id) return savedThread
        foundThread = true
        const hasMessage = savedThread.messages.some((message) => message.id === assistantMessage.id)
        return {
          ...savedThread,
          running: false,
          updatedAt: thread.updatedAt,
          lastRunError: thread.lastRunError,
          unreadCount: thread.unreadCount,
          messages: hasMessage
            ? savedThread.messages.map((message) => message.id === assistantMessage.id ? persistedMessage : message)
            : [...savedThread.messages, persistedMessage]
        }
      })
    }
    if (!foundThread) return
    await savePlaygroundImagesToDB(imageRecords)
    await savePlaygroundStateToDB(nextPayload)
    localStorage.setItem(storageKey(), JSON.stringify(buildLocalStoragePayload(nextPayload)))
    window.dispatchEvent(new CustomEvent(PLAYGROUND_STATE_UPDATED_EVENT, { detail: { key: storageKey(), source: playgroundInstanceId } }))
  } catch (error) {
    console.warn('Failed to persist completed playground image message:', error)
  }
}

function persistPlaygroundState() {
  if (restoringState || !persistenceReady) return
  if (persistTimer !== undefined) window.clearTimeout(persistTimer)
  persistTimer = window.setTimeout(() => {
    void writePlaygroundStateNow()
  }, 180)
}

async function restorePlaygroundState() {
  restoringState = true
  try {
    const dbPayload = await loadPlaygroundStateFromDB().catch((error) => {
      console.warn('Failed to load playground state from IndexedDB:', error)
      return null
    })
    if (dbPayload) {
      applyPlaygroundPayload(dbPayload as unknown as Record<string, unknown>)
      await hydratePersistedImages()
      return
    }
    const raw = localStorage.getItem(storageKey())
    if (!raw) return
    applyPlaygroundPayload(JSON.parse(raw) as Record<string, unknown>)
    await hydratePersistedImages()
  } catch (error) {
    console.warn('Failed to restore playground state:', error)
  } finally {
    restoringState = false
  }
}

async function refreshPlaygroundStateFromStorage() {
  const activeIdBeforeRefresh = activeThreadId.value
  const modeBeforeRefresh = mode.value
  const payload = await loadPlaygroundStateFromDB().catch((error) => {
    console.warn('Failed to refresh playground state:', error)
    return null
  })
  if (!payload) return
  restoringState = true
  try {
    revokeAllImageObjectURLs()
    applyPlaygroundPayload(payload as unknown as Record<string, unknown>)
    await hydratePersistedImages()
    if (threads.value.some((thread) => thread.id === activeIdBeforeRefresh)) {
      activeThreadId.value = activeIdBeforeRefresh
      mode.value = modeBeforeRefresh
    }
  } finally {
    restoringState = false
  }
  await nextTick()
  updateComposerSpacer()
  scrollMessagesToBottom()
}

function handlePlaygroundStateUpdated(event: Event) {
  const detail = (event as CustomEvent<{ key?: string; source?: string }>).detail
  if (detail?.key !== storageKey() || detail?.source === playgroundInstanceId) return
  void refreshPlaygroundStateFromStorage()
}

function modeIcon(value: PlaygroundMode): IconName {
  if (value === 'image') return 'sparkles'
  if (value === 'video') return 'play'
  if (value === 'audio') return 'cloud'
  return 'chat'
}

function modeLabel(value: PlaygroundMode): string {
  return modeOptions.value.find((item) => item.value === value)?.label || value
}

function threadPreview(thread: PlaygroundThread): string {
  const last = [...thread.messages].reverse().find((message) => message.content || message.images?.length)
  if (!last) return t('playground.emptyConversation')
  if (last.images?.length) return t('playground.imageResultCount', { count: last.images.length })
  return last.content
}

function formatThreadDate(timestamp: number): string {
  const date = new Date(timestamp)
  const now = new Date()
  if (date.toDateString() === now.toDateString()) {
    return formatRelativeTime(date)
  }
  return formatDateOnly(date)
}

function formatMessageTime(timestamp: number): string {
  return formatTime(new Date(timestamp))
}

function renderMessageMarkdown(content: string): string {
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
}

function imageMessageSizeLabel(message: PlaygroundMessage): string {
  const size = message.imageConfig?.size?.trim()
  if (size) return size
  return message.images?.length ? t('playground.imageResultCount', { count: message.images.length }) : '-'
}

function formatImageGenerationDuration(durationMs?: number): string {
  if (typeof durationMs !== 'number' || !Number.isFinite(durationMs) || durationMs < 0) return '-'
  if (durationMs < 1000) return `${Math.max(1, Math.round(durationMs))}ms`
  const seconds = durationMs / 1000
  if (seconds < 10) return `${seconds.toFixed(1)}s`
  if (seconds < 60) return `${Math.round(seconds)}s`
  const minutes = Math.floor(seconds / 60)
  const restSeconds = Math.round(seconds % 60)
  return `${minutes}m ${restSeconds}s`
}

function imagePromptStyleLabel(style: ImagePromptStyle = imagePromptStyle.value): string {
  const option = imagePromptStyleSelectOptions.value.find((item) => item.value === style)
  return option?.label.replace(`${t('playground.promptStyle')}：`, '') || t('playground.promptStyleAuto')
}

function promptStyleOptionLabel(option: SelectOption): string {
  return String(option.label).replace(`${t('playground.promptStyle')}：`, '')
}

function messageDisplayName(message: PlaygroundMessage): string {
  if (message.role === 'user') return userDisplayName.value
  return message.model || t('playground.assistant')
}

function createThread(nextMode: PlaygroundMode = mode.value): PlaygroundThread {
  const thread = createEmptyThread(nextMode)
  threads.value.unshift(thread)
  activeThreadId.value = thread.id
  mode.value = nextMode
  selectDefaultModel()
  return thread
}

function ensureActiveThread(): PlaygroundThread {
  return activeThread.value || createThread(mode.value)
}

function selectThread(id: string) {
  const thread = threads.value.find((item) => item.id === id)
  if (!thread) return
  rememberSelectedModelForMode()
  activeThreadId.value = id
  thread.unreadCount = 0
  mode.value = thread.mode
  selectDefaultModel()
  scrollMessagesToBottom()
}

function deleteThread(id: string) {
  const index = threads.value.findIndex((thread) => thread.id === id)
  if (index === -1) return
  const deleted = threads.value[index]
  for (const message of deleted.messages) {
    revokeImageObjectURLs(message.images)
  }
  threads.value.splice(index, 1)
  if (activeThreadId.value === id) {
    const nextThread = threads.value.find((thread) => thread.mode === deleted.mode) || threads.value[0]
    if (nextThread) {
      activeThreadId.value = nextThread.id
      mode.value = nextThread.mode
    } else {
      createThread(mode.value)
    }
  }
}

function clearHistory() {
  threads.value = threads.value.filter((thread) => thread.mode !== mode.value)
  createThread(mode.value)
}

async function buildConversationExportPayload(): Promise<PlaygroundConversationExportPayload> {
  const exportThreads = modeThreads.value.map(cloneThreadForTransfer)
  const images: PlaygroundConversationExportImage[] = []
  const exportedImageIds = new Set<string>()

  for (const thread of exportThreads) {
    thread.running = false
    thread.lastRunError = ''
    thread.pendingAttachments = []
    for (const message of thread.messages) {
      message.pending = false
      message.progress = ''
      message.raw = undefined
      for (const image of message.images || []) {
        const originalUrl = image.url || ''
        const storageId = image.storageId || storedImageIdFromURL(originalUrl)
        if (!storageId) continue
        image.storageId = storageId
        image.url = storedImagePlaceholder(storageId)
        image.thumbnailUrl = undefined
        if (exportedImageIds.has(storageId)) continue
        exportedImageIds.add(storageId)
        const exportedImage: PlaygroundConversationExportImage = {
          id: storageId,
          mimeType: image.mimeType,
          revisedPrompt: image.revisedPrompt
        }
        const persisted = await loadPlaygroundImageFromDB(storageId).catch(() => null)
        if (persisted?.blob) {
          exportedImage.dataUrl = await blobToDataURL(persisted.blob)
          exportedImage.mimeType = exportedImage.mimeType || persisted.mimeType || persisted.blob.type
          exportedImage.revisedPrompt = exportedImage.revisedPrompt || persisted.revisedPrompt
        } else if (originalUrl.startsWith('data:')) {
          exportedImage.dataUrl = originalUrl
        } else if (persisted?.url) {
          exportedImage.url = persisted.url
          exportedImage.mimeType = exportedImage.mimeType || persisted.mimeType
          exportedImage.revisedPrompt = exportedImage.revisedPrompt || persisted.revisedPrompt
        } else if (originalUrl && !originalUrl.startsWith(PLAYGROUND_IMAGE_URL_PREFIX) && !originalUrl.startsWith('blob:')) {
          exportedImage.url = originalUrl
        }
        if (exportedImage.dataUrl || exportedImage.url) {
          images.push(exportedImage)
        }
      }
    }
  }

  return {
    type: 'sub2api-playground-conversations',
    version: 1,
    exportedAt: Date.now(),
    mode: mode.value,
    threads: exportThreads,
    images
  }
}

async function exportConversations() {
  if (modeThreads.value.length === 0) {
    appStore.showInfo(t('playground.noConversationsToExport'))
    return
  }
  try {
    await writePlaygroundStateNow()
    const payload = await buildConversationExportPayload()
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    const date = new Date().toISOString().slice(0, 10)
    link.href = url
    link.download = `moosecloud-playground-${mode.value}-${date}.json`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    appStore.showSuccess(t('playground.conversationsExported', { count: payload.threads.length }))
  } catch (error) {
    console.warn('Failed to export playground conversations:', error)
    appStore.showError(t('playground.conversationExportFailed'))
  }
}

function extractConversationImportPayload(rawPayload: unknown): {
  mode?: PlaygroundMode
  threads: PlaygroundThread[]
  images: Map<string, PlaygroundConversationExportImage>
} {
  if (!isRecord(rawPayload)) throw new Error('Invalid import payload')
  const payloadThreads = Array.isArray(rawPayload.threads) ? rawPayload.threads : []
  if (payloadThreads.length === 0) throw new Error('No conversations in import payload')
  const images = new Map<string, PlaygroundConversationExportImage>()
  if (Array.isArray(rawPayload.images)) {
    for (const image of rawPayload.images) {
      if (!isRecord(image) || typeof image.id !== 'string') continue
      images.set(image.id, {
        id: image.id,
        dataUrl: typeof image.dataUrl === 'string' ? image.dataUrl : undefined,
        url: typeof image.url === 'string' ? image.url : undefined,
        mimeType: typeof image.mimeType === 'string' ? image.mimeType : undefined,
        revisedPrompt: typeof image.revisedPrompt === 'string' ? image.revisedPrompt : undefined
      })
    }
  }
  return {
    mode: isPlaygroundMode(rawPayload.mode) ? rawPayload.mode : undefined,
    threads: payloadThreads as PlaygroundThread[],
    images
  }
}

async function importConversationsFromPayload(rawPayload: unknown) {
  const payload = extractConversationImportPayload(rawPayload)
  const imageRecords: PlaygroundPersistedImage[] = []
  const importedThreads = await Promise.all(
    payload.threads.map((thread) => rewriteImportedThreadIds(thread, payload.images, imageRecords))
  )
  const validThreads = importedThreads.filter((thread) => isPlaygroundMode(thread.mode))
  if (validThreads.length === 0) {
    throw new Error('No valid conversations in import payload')
  }
  if (imageRecords.length > 0) {
    await savePlaygroundImagesToDB(imageRecords)
  }
  threads.value = [...validThreads, ...threads.value]
  const preferredThread = validThreads.find((thread) => thread.mode === mode.value) || validThreads[0]
  activeThreadId.value = preferredThread.id
  mode.value = preferredThread.mode
  selectDefaultModel()
  await writePlaygroundStateNow()
  await nextTick()
  scrollMessagesToBottom()
  appStore.showSuccess(t('playground.conversationsImported', { count: validThreads.length }))
}

async function handleConversationImportChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    const text = await file.text()
    const payload = JSON.parse(text)
    await importConversationsFromPayload(payload)
  } catch (error) {
    console.warn('Failed to import playground conversations:', error)
    appStore.showError(t('playground.conversationImportFailed'))
  } finally {
    input.value = ''
  }
}

function selectMode(nextMode: PlaygroundMode) {
  rememberSelectedModelForMode()
  mode.value = nextMode
  const existing = threads.value.find((thread) => thread.mode === nextMode)
  if (existing) {
    activeThreadId.value = existing.id
  } else {
    createThread(nextMode)
  }
  selectDefaultModel()
}

function rememberSelectedModelForMode(targetMode: PlaygroundMode = mode.value) {
  if (selectedModel.value.trim()) {
    selectedModelsByMode.value = {
      ...selectedModelsByMode.value,
      [targetMode]: selectedModel.value.trim()
    }
  }
}

function selectDefaultModel() {
  const remembered = selectedModelsByMode.value[mode.value]
  if (remembered && modelAvailable(remembered)) {
    selectedModel.value = remembered
    return
  }
  const preferred = preferredModelForMode()
  if (selectedModel.value && modelAvailable(selectedModel.value)) {
    rememberSelectedModelForMode()
    return
  }
  if (preferred) {
    selectedModel.value = preferred
    rememberSelectedModelForMode()
    return
  }
  selectedModel.value = visibleModels.value[0]?.id || ''
  rememberSelectedModelForMode()
}

function selectDefaultPromptOptimizerModel() {
  if (promptOptimizerModel.value && chatModels.value.some((model) => model.id === promptOptimizerModel.value)) return
  const preferred = selectedKeyPlatform.value === 'openai'
    ? 'gpt-5.5'
    : isClaudePlatform(selectedKeyPlatform.value)
      ? 'claude-opus-4-8'
      : ''
  if (preferred && chatModels.value.some((model) => model.id === preferred)) {
    promptOptimizerModel.value = preferred
    return
  }
  promptOptimizerModel.value = chatModels.value[0]?.id || ''
}

function buildThreadTitle(prompt: string): string {
  const compact = prompt.replace(/\s+/g, ' ').trim()
  return compact ? compact.slice(0, 32) : modeLabel(mode.value)
}

function currentImageConfig(): PlaygroundImageConfig {
  return {
    sizeMode: imageSizeMode.value,
    resolution: imageResolution.value,
    ratio: imageRatio.value,
    customWidth: clampImageDimension(customImageWidth.value),
    customHeight: clampImageDimension(customImageHeight.value),
    size: effectiveImageSize.value,
    quality: imageQuality.value,
    outputFormat: outputFormat.value,
    count: Math.min(Math.max(Number(imageCount.value) || 1, 1), 4)
  }
}

function reuseImageConfig(message: PlaygroundMessage) {
  if (!message.imageConfig) return
  imageSizeMode.value = message.imageConfig.sizeMode
  imageResolution.value = message.imageConfig.resolution
  imageRatio.value = message.imageConfig.ratio
  customImageWidth.value = message.imageConfig.customWidth
  customImageHeight.value = message.imageConfig.customHeight
  imageQuality.value = message.imageConfig.quality
  outputFormat.value = message.imageConfig.outputFormat
  imageCount.value = message.imageConfig.count
  mode.value = 'image'
  if (message.model) selectedModel.value = message.model
  if (activeThread.value) activeThread.value.mode = 'image'
  appStore.showSuccess(t('playground.imageConfigReused'))
}

function clampImagePreviewZoom(value: number): number {
  return Math.min(Math.max(value, 0.2), 8)
}

function imagePreviewPanLimit() {
  const overflowX = Math.max(0, (imagePreviewDisplaySize.value.width - imagePreviewViewportWidth.value) / 2)
  const overflowY = Math.max(0, (imagePreviewDisplaySize.value.height - imagePreviewViewportHeight.value) / 2)
  return {
    x: overflowX,
    y: overflowY
  }
}

function clampImagePreviewPan() {
  const limit = imagePreviewPanLimit()
  imagePreviewPanX.value = Math.min(Math.max(imagePreviewPanX.value, -limit.x), limit.x)
  imagePreviewPanY.value = Math.min(Math.max(imagePreviewPanY.value, -limit.y), limit.y)
}

function updateImagePreviewViewportSize() {
  const viewport = imagePreviewViewport.value
  if (!viewport) return
  imagePreviewViewportWidth.value = viewport.clientWidth
  imagePreviewViewportHeight.value = viewport.clientHeight
  clampImagePreviewPan()
}

function handleImagePreviewLoad(event: Event) {
  const image = event.target as HTMLImageElement
  imagePreviewNaturalWidth.value = image.naturalWidth || 0
  imagePreviewNaturalHeight.value = image.naturalHeight || 0
  updateImagePreviewViewportSize()
  resetImagePreviewView()
}

function handleImagePreviewWheel(event: WheelEvent) {
  const viewport = imagePreviewViewport.value
  if (!viewport) return
  const previousZoom = imagePreviewZoom.value
  const zoomFactor = event.deltaY < 0 ? 1.12 : 1 / 1.12
  const nextZoom = clampImagePreviewZoom(Number((previousZoom * zoomFactor).toFixed(3)))
  if (nextZoom === previousZoom) return

  const rect = viewport.getBoundingClientRect()
  const pointerX = event.clientX - rect.left - rect.width / 2
  const pointerY = event.clientY - rect.top - rect.height / 2
  const scaleRatio = nextZoom / previousZoom
  imagePreviewPanX.value = pointerX - (pointerX - imagePreviewPanX.value) * scaleRatio
  imagePreviewPanY.value = pointerY - (pointerY - imagePreviewPanY.value) * scaleRatio
  imagePreviewZoom.value = nextZoom
  clampImagePreviewPan()
}

function handleImagePreviewPointerDown(event: PointerEvent) {
  if (event.button !== 0) return
  const viewport = imagePreviewViewport.value
  if (!viewport) return
  event.preventDefault()
  viewport.setPointerCapture(event.pointerId)
  imagePreviewDragState = {
    pointerId: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    panX: imagePreviewPanX.value,
    panY: imagePreviewPanY.value
  }
  imagePreviewIsDragging.value = true
}

function handleImagePreviewPointerMove(event: PointerEvent) {
  if (!imagePreviewDragState || imagePreviewDragState.pointerId !== event.pointerId) return
  imagePreviewPanX.value = imagePreviewDragState.panX + event.clientX - imagePreviewDragState.startX
  imagePreviewPanY.value = imagePreviewDragState.panY + event.clientY - imagePreviewDragState.startY
  clampImagePreviewPan()
}

function handleImagePreviewPointerUp(event: PointerEvent) {
  if (!imagePreviewDragState || imagePreviewDragState.pointerId !== event.pointerId) return
  const viewport = imagePreviewViewport.value
  if (viewport?.hasPointerCapture(event.pointerId)) {
    viewport.releasePointerCapture(event.pointerId)
  }
  imagePreviewDragState = null
  imagePreviewIsDragging.value = false
}

function resetImagePreviewView() {
  imagePreviewZoom.value = 1
  imagePreviewPanX.value = 0
  imagePreviewPanY.value = 0
  imagePreviewDragState = null
  imagePreviewIsDragging.value = false
}

async function openImagePreview(image: PlaygroundStoredImageResult, index: number) {
  closeImagePreview()
  imagePreviewZoom.value = 1
  imagePreviewPanX.value = 0
  imagePreviewPanY.value = 0
  imagePreviewNaturalWidth.value = 0
  imagePreviewNaturalHeight.value = 0
  const title = t('playground.generatedImageAlt', { n: index + 1 })
  const storageId = image.storageId || storedImageIdFromURL(image.url)
  if (storageId) {
    const persisted = await loadPlaygroundImageFromDB(storageId).catch((error) => {
      console.warn('Failed to load original playground image:', error)
      return null
    })
    if (persisted?.blob) {
      imagePreview.value = {
        url: createTrackedObjectURL(persisted.blob),
        title,
        revisedPrompt: image.revisedPrompt || persisted.revisedPrompt
      }
      return
    }
    if (persisted?.url) {
      imagePreview.value = {
        url: persisted.url,
        title,
        revisedPrompt: image.revisedPrompt || persisted.revisedPrompt
      }
      return
    }
  }
  if (image.url && !image.url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX)) {
    imagePreview.value = {
      url: image.url,
      title,
      revisedPrompt: image.revisedPrompt
    }
    return
  }
  appStore.showError(t('playground.imageCacheMissing'))
}

function closeImagePreview() {
  if (imagePreview.value?.url.startsWith('blob:')) {
    revokeTrackedObjectURL(imagePreview.value.url)
  }
  imagePreview.value = null
  resetImagePreviewView()
  imagePreviewNaturalWidth.value = 0
  imagePreviewNaturalHeight.value = 0
}

function deleteMessage(messageId: string) {
  const thread = activeThread.value
  if (!thread) return
  const deleted = thread.messages.find((message) => message.id === messageId)
  revokeImageObjectURLs(deleted?.images)
  thread.messages = thread.messages.filter((message) => message.id !== messageId)
  thread.updatedAt = Date.now()
}

async function scrollMessagesToBottom() {
  await nextTick()
  if (messageScroller.value) {
    messageScroller.value.scrollTop = messageScroller.value.scrollHeight
  }
}

function isMessageScrollerNearBottom(threshold = 96): boolean {
  const scroller = messageScroller.value
  if (!scroller) return true
  return scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight <= threshold
}

async function scrollMessagesToBottomIfNeeded(shouldScroll: boolean) {
  if (!shouldScroll) return
  await scrollMessagesToBottom()
}

function updateComposerSpacer() {
  const dockHeight = composerDock.value?.offsetHeight || 0
  composerSpacerHeight.value = Math.max(260, dockHeight + 40)
}

function handleComposerInputResizePointerDown(event: PointerEvent) {
  event.preventDefault()
  composerInputResizeState = {
    pointerId: event.pointerId,
    startY: event.clientY,
    height: composerInputHeight.value
  }
  composerInputResizing.value = true
  window.addEventListener('pointermove', handleComposerInputResizePointerMove)
  window.addEventListener('pointerup', handleComposerInputResizePointerUp)
  window.addEventListener('pointercancel', handleComposerInputResizePointerUp)
}

function handleComposerInputResizePointerMove(event: PointerEvent) {
  if (!composerInputResizeState || composerInputResizeState.pointerId !== event.pointerId) return
  const delta = composerInputResizeState.startY - event.clientY
  composerInputHeight.value = clampComposerInputHeight(composerInputResizeState.height + delta)
  updateComposerSpacer()
}

function handleComposerInputResizePointerUp(event: PointerEvent) {
  if (composerInputResizeState && composerInputResizeState.pointerId !== event.pointerId) return
  composerInputResizeState = null
  composerInputResizing.value = false
  window.removeEventListener('pointermove', handleComposerInputResizePointerMove)
  window.removeEventListener('pointerup', handleComposerInputResizePointerUp)
  window.removeEventListener('pointercancel', handleComposerInputResizePointerUp)
  updateComposerSpacer()
  persistPlaygroundState()
}

async function loadKeys() {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 200, { status: 'active' })
    apiKeys.value = response.items
    if (!selectedKeyId.value && activeKeys.value.length > 0) {
      selectedKeyId.value = String(activeKeys.value[0].id)
    }
  } catch (error) {
    appStore.showError((error as Error)?.message || t('playground.loadKeysFailed'))
  } finally {
    loadingKeys.value = false
  }
}

async function loadAvailableChannels() {
  try {
    availableChannels.value = await userChannelsAPI.getAvailable()
    selectDefaultModel()
  } catch (error) {
    console.warn('Failed to load available channels for playground:', error)
  }
}

async function loadModels() {
  if (!selectedKey.value) return
  modelAbortController?.abort()
  const controller = new AbortController()
  modelAbortController = controller
  loadingModels.value = true
  modelLoadError.value = ''
  try {
    const fetched = await fetchModels(selectedKey.value.key, selectedEndpoint.value?.requestBase, controller.signal)
    if (controller.signal.aborted) return
    models.value = fetched
    selectDefaultModel()
    selectDefaultPromptOptimizerModel()
  } catch (error) {
    if (controller.signal.aborted) return
    modelLoadError.value = (error as Error)?.message || t('playground.loadModelsFailed')
    appStore.showError(modelLoadError.value)
    selectDefaultModel()
    selectDefaultPromptOptimizerModel()
  } finally {
    if (modelAbortController === controller) {
      loadingModels.value = false
    }
  }
}

function validateRun(): boolean {
  if (!selectedKey.value) {
    appStore.showInfo(t('playground.selectKeyFirst'))
    return false
  }
  if (!selectedEndpoint.value) {
    appStore.showInfo(t('playground.selectEndpointFirst'))
    return false
  }
  if (!effectiveModel.value) {
    appStore.showInfo(t('playground.selectModelFirst'))
    return false
  }
  if (!draftPrompt.value.trim() && pendingAttachments.value.length === 0) {
    appStore.showInfo(mode.value === 'image' ? t('playground.enterImagePrompt') : t('playground.enterPrompt'))
    return false
  }
  return true
}

function validatePromptOptimizer(): boolean {
  if (!selectedKey.value) {
    appStore.showInfo(t('playground.selectKeyFirst'))
    return false
  }
  if (!selectedEndpoint.value) {
    appStore.showInfo(t('playground.selectEndpointFirst'))
    return false
  }
  if (!promptOptimizerModel.value.trim()) {
    appStore.showInfo(t('playground.selectOptimizerModelFirst'))
    return false
  }
  if (!draftPrompt.value.trim()) {
    appStore.showInfo(t('playground.enterImagePrompt'))
    return false
  }
  return true
}

function markRunErrorHandled(error: unknown, message: string): Error {
  const handledError = error instanceof Error ? error : new Error(message)
  Object.defineProperty(handledError, '__playgroundHandled', {
    value: true,
    configurable: true
  })
  return handledError
}

function isHandledRunError(error: unknown): boolean {
  return Boolean(error && typeof error === 'object' && (error as Record<string, unknown>).__playgroundHandled)
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  if (signal?.aborted) return Promise.reject(new DOMException('Aborted', 'AbortError'))
  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    const onAbort = () => {
      window.clearTimeout(timeout)
      reject(new DOMException('Aborted', 'AbortError'))
    }
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

function apiKeyForRunMessage(message: PlaygroundMessage): string {
  const keyId = message.runKeyId || selectedKeyId.value
  const key = activeKeys.value.find((item) => String(item.id) === keyId)
  return key?.key || ''
}

function isTerminalPlaygroundRun(run: PlaygroundRun): boolean {
  return run.status === 'succeeded' || run.status === 'failed' || run.status === 'canceled'
}

function buildRunError(run: PlaygroundRun): Error {
  return new Error(run.error || (run.status === 'canceled' ? t('playground.requestStopped') : t('playground.runFailed')))
}

function buildChatRunRequest(
  runId: string,
  prompt: string,
  attachments: PlaygroundAttachment[],
  history: PlaygroundMessage[],
  context: PlaygroundRunContext
): PlaygroundRunRequest {
  return {
    id: runId,
    mode: 'chat',
    apiKey: context.apiKey,
    endpointBase: context.endpointBase,
    displayEndpoint: context.displayEndpoint,
    model: context.model,
    messages: buildChatMessages(prompt, attachments, history, context.mode),
    temperature: context.temperature,
    topP: context.topP,
    maxTokens: context.maxTokens,
    presencePenalty: context.presencePenalty,
    frequencyPenalty: context.frequencyPenalty
  }
}

function buildImageRunRequest(
  runId: string,
  prompt: string,
  context: PlaygroundRunContext
): PlaygroundRunRequest {
  return {
    id: runId,
    mode: 'image',
    apiKey: context.apiKey,
    endpointBase: context.endpointBase,
    displayEndpoint: context.displayEndpoint,
    model: context.model,
    prompt,
    size: context.imageSize,
    n: context.imageCount,
    quality: context.imageQuality,
    outputFormat: context.outputFormat
  }
}

function stripRunAPIKey(request: PlaygroundRunRequest): PlaygroundRestorableRunRequest {
  const { apiKey: _apiKey, ...rest } = request
  return rest
}

async function ensureBackendPlaygroundRun(
  request: PlaygroundRunRequest,
  controller: AbortController
): Promise<PlaygroundRun> {
  try {
    return await startPlaygroundRun(request)
  } catch (error) {
    if (controller.signal.aborted) throw error
    throw error
  }
}

async function pollPlaygroundRun(
  runId: string,
  apply: (run: PlaygroundRun) => Promise<void> | void,
  controller: AbortController
): Promise<PlaygroundRun> {
  const startedAt = Date.now()
  while (true) {
    if (controller.signal.aborted) throw new DOMException('Aborted', 'AbortError')
    const run = await getPlaygroundRun(runId)
    await apply(run)
    if (isTerminalPlaygroundRun(run)) return run
    if (Date.now() - startedAt > PLAYGROUND_RUN_POLL_MAX_MS) {
      throw new Error(t('playground.runTimeout'))
    }
    await sleep(PLAYGROUND_RUN_POLL_INTERVAL_MS, controller.signal)
  }
}

async function applyCompletedChatRun(
  thread: PlaygroundThread,
  message: PlaygroundMessage,
  run: PlaygroundRun
) {
  if (run.status !== 'succeeded') {
    message.pending = false
    message.progress = ''
    message.error = true
    message.content = run.error || t('playground.runFailed')
    thread.lastRunError = message.content
    thread.updatedAt = Date.now()
    await writePlaygroundStateNow()
    return
  }
  message.pending = false
  message.progress = ''
  message.error = false
  message.content = run.content || message.content || t('playground.emptyTextResponse')
  message.raw = run.raw
  message.durationMs = run.durationMs
  thread.updatedAt = Date.now()
  await writePlaygroundStateNow()
}

async function applyCompletedImageRun(
  thread: PlaygroundThread,
  message: PlaygroundMessage,
  run: PlaygroundRun
) {
  if (run.status !== 'succeeded') {
    message.pending = false
    message.progress = ''
    message.error = true
    message.content = run.error || t('playground.runFailed')
    thread.lastRunError = message.content
    thread.updatedAt = Date.now()
    await persistCompletedImageMessage(thread, message)
    return
  }
  const images = run.images || []
  message.pending = false
  message.progress = ''
  message.error = false
  message.durationMs = run.durationMs
  message.content = images.length > 0 ? t('playground.imageGenerated') : t('playground.noImageReturned')
  message.images = images.map((image, index) => ({
    ...image,
    storageId: uid(`image-${message.id}-${index}`),
    mimeType: image.url.startsWith('data:')
      ? image.url.match(/^data:([^;,]+)/)?.[1]
      : undefined
  }))
  message.raw = run.raw
  thread.updatedAt = Date.now()
  await persistCompletedImageMessage(thread, message)
}

async function resumePlaygroundRun(thread: PlaygroundThread, message: PlaygroundMessage) {
  if (!message.pending || !message.runId || runAbortControllers.has(thread.id)) return
  const controller = new AbortController()
  const runMode: PlaygroundMode = message.imageConfig ? 'image' : thread.mode
  runAbortControllers.set(thread.id, { controller, mode: runMode, runId: message.runId })
  thread.running = true
  try {
    const request = message.runRequest
    const apiKey = apiKeyForRunMessage(message)
    if (request && apiKey) {
      await ensureBackendPlaygroundRun({ ...request, apiKey } as PlaygroundRunRequest, controller)
    }
    const finalRun = await pollPlaygroundRun(message.runId, async (run) => {
      if (run.mode === 'chat' && typeof run.content === 'string') {
        message.content = run.content
      }
      message.progress = run.mode === 'image'
        ? (run.status === 'queued' ? t('playground.waiting') : t('playground.generatingImages'))
        : (run.status === 'queued' ? t('playground.waiting') : t('playground.streaming'))
      thread.updatedAt = Date.now()
      await writePlaygroundStateNow()
    }, controller)

    if (finalRun.mode === 'image' || message.imageConfig) {
      await applyCompletedImageRun(thread, message, finalRun)
    } else {
      await applyCompletedChatRun(thread, message, finalRun)
    }
    if (activeThreadId.value !== thread.id) {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
  } catch (error) {
    if (controller.signal.aborted) return
    message.pending = false
    message.progress = ''
    message.error = true
    message.content = (error as Error)?.message || t('playground.runFailed')
    thread.lastRunError = message.content
    thread.updatedAt = Date.now()
    if (message.imageConfig) {
      await persistCompletedImageMessage(thread, message)
    } else {
      await writePlaygroundStateNow()
    }
  } finally {
    if (runAbortControllers.get(thread.id)?.controller === controller) {
      runAbortControllers.delete(thread.id)
      thread.running = false
    }
  }
}

function resumePendingPlaygroundRuns() {
  for (const thread of threads.value) {
    const pendingMessage = [...thread.messages].reverse().find((message) => message.pending && message.runId)
    if (!pendingMessage) {
      thread.running = false
      continue
    }
    void resumePlaygroundRun(thread, pendingMessage)
  }
}

function buildImagePromptOptimizerMessages(prompt: string): PlaygroundChatMessage[] {
  const styleLabel = imagePromptStyleLabel()
  const targetStyle = imagePromptStyle.value === 'auto'
    ? t('playground.optimizerStyleAutoInstruction')
    : t('playground.optimizerStyleInstruction', { style: styleLabel })
  return [
    {
      role: 'system',
      content: t('playground.promptOptimizerSystem')
    },
    {
      role: 'user',
      content: [
        t('playground.promptOptimizerTask'),
        `${t('playground.promptStyle')}: ${styleLabel}`,
        targetStyle,
        `${t('playground.imageSize')}: ${effectiveImageSize.value}`,
        `${t('playground.quality')}: ${imageQuality.value}`,
        `${t('playground.outputFormat')}: ${outputFormat.value.toUpperCase()}`,
        '',
        prompt
      ].join('\n')
    }
  ]
}

async function optimizeImagePrompt() {
  if (optimizingPrompt.value || !validatePromptOptimizer() || !selectedKey.value || !selectedEndpoint.value) return
  promptOptimizeAbortController?.abort()
  const controller = new AbortController()
  promptOptimizeAbortController = controller
  optimizingPrompt.value = true
  const originalPrompt = draftPrompt.value.trim()
  let optimized = ''
  try {
    const response = await streamChatCompletion({
      apiKey: selectedKey.value.key,
      endpointBase: selectedEndpoint.value.requestBase,
      displayEndpoint: selectedEndpoint.value.value,
      model: promptOptimizerModel.value.trim(),
      messages: buildImagePromptOptimizerMessages(originalPrompt),
      temperature: 0.4,
      topP: 0.9,
      maxTokens: 1200,
      signal: controller.signal,
      onDelta(delta) {
        optimized += delta
        draftPrompt.value = optimized.trimStart()
      }
    })
    if (controller.signal.aborted) return
    optimized = (optimized || response.content || '').trim()
    if (!optimized) throw new Error(t('playground.promptOptimizeEmpty'))
    draftPrompt.value = stripPromptOptimizerFences(optimized)
    showPromptOptimizerModal.value = false
    appStore.showSuccess(t('playground.promptOptimized'))
  } catch (error) {
    if (controller.signal.aborted) return
    draftPrompt.value = originalPrompt
    appStore.showError((error as Error)?.message || t('playground.promptOptimizeFailed'))
  } finally {
    if (promptOptimizeAbortController === controller) {
      promptOptimizeAbortController = null
      optimizingPrompt.value = false
    }
  }
}

function stripPromptOptimizerFences(value: string): string {
  return value
    .replace(/^```(?:text|markdown|prompt)?\s*/i, '')
    .replace(/\s*```$/i, '')
    .trim()
}

function attachmentPrompt(attachments: PlaygroundAttachment[]): string {
  return attachments
    .filter((attachment) => attachment.kind !== 'image')
    .map((attachment) => {
      if (attachment.text) return `\n\n[${attachment.name}]\n${attachment.text}`
      return `\n\n[${attachment.name}] ${t('playground.binaryAttachmentNote')}`
    })
    .join('')
}

function buildUserMessageContent(prompt: string, attachments: PlaygroundAttachment[]): PlaygroundChatMessage['content'] {
  const imageAttachments = attachments.filter((attachment) => attachment.kind === 'image' && attachment.dataUrl)
  const text = `${prompt}${attachmentPrompt(attachments)}`.trim()
  if (imageAttachments.length === 0) return text
  return [
    { type: 'text', text: text || t('playground.attachedImagesOnly') },
    ...imageAttachments.map((attachment) => ({
      type: 'image_url' as const,
      image_url: { url: attachment.dataUrl || '' }
    }))
  ]
}

function modeSystemInstruction(targetMode: PlaygroundMode = mode.value): string {
  if (targetMode === 'video') return t('playground.videoSystemPrompt')
  if (targetMode === 'audio') return t('playground.audioSystemPrompt')
  return ''
}

function buildChatMessages(
  prompt: string,
  attachments: PlaygroundAttachment[],
  history: PlaygroundMessage[],
  targetMode: PlaygroundMode
): PlaygroundChatMessage[] {
  const messages: PlaygroundChatMessage[] = []
  const systemParts = [systemPrompt.value.trim(), modeSystemInstruction(targetMode)].filter(Boolean)
  if (systemParts.length > 0) {
    messages.push({ role: 'system', content: systemParts.join('\n\n') })
  }
  for (const message of history) {
    if (!message.content || message.pending || message.images?.length) continue
    messages.push({ role: message.role, content: message.content })
  }
  messages.push({ role: 'user', content: buildUserMessageContent(prompt, attachments) })
  return messages
}

async function submitPrompt() {
  const thread = ensureActiveThread()
  if (thread.running || !validateRun() || !selectedKey.value || !selectedEndpoint.value) return
  lastRunError.value = ''
  const context: PlaygroundRunContext = {
    mode: thread.mode,
    keyId: selectedKeyId.value,
    apiKey: selectedKey.value.key,
    endpointBase: selectedEndpoint.value?.requestBase,
    displayEndpoint: selectedEndpoint.value?.value,
    model: effectiveModel.value,
    platform: platformForModel(effectiveModel.value),
    temperature: temperature.value,
    topP: topP.value,
    maxTokens: maxTokens.value,
    presencePenalty: presencePenalty.value,
    frequencyPenalty: frequencyPenalty.value,
    imageSize: effectiveImageSize.value,
    imageCount: Math.min(Math.max(Number(imageCount.value) || 1, 1), 4),
    imageQuality: imageQuality.value,
    outputFormat: outputFormat.value
  }
  const imageConfig = currentImageConfig()

  const prompt = draftPrompt.value.trim()
  const attachments = [...pendingAttachments.value]
  const runId = uid('run')
  const userMessage: PlaygroundMessage = {
    id: uid('msg'),
    role: 'user',
    content: prompt || t('playground.attachmentOnlyPrompt'),
    createdAt: Date.now(),
    attachments
  }
  thread.messages.push(userMessage)
  if (thread.messages.length === 1) thread.title = buildThreadTitle(prompt)
  thread.updatedAt = Date.now()
  lastPrompt.value = prompt

  draftPrompt.value = ''
  pendingAttachments.value = []
  const controller = new AbortController()
  runAbortControllers.set(thread.id, { controller, mode: context.mode, runId })
  thread.running = true
  await scrollMessagesToBottom()

  try {
    if (context.mode === 'image') {
      await runImageGeneration(thread, prompt, context, imageConfig, controller, runId)
    } else {
      await runStreamingChat(thread, prompt, attachments, context, controller, runId)
    }
    if (activeThreadId.value === thread.id) {
      appStore.showSuccess(t('playground.runSuccess'))
    }
  } catch (error) {
    if (controller.signal.aborted) return
    const handledInMessage = isHandledRunError(error)
    const errorMessage = (error as Error)?.message || t('playground.runFailed')
    if (!handledInMessage) {
      const assistantMessage: PlaygroundMessage = {
        id: uid('msg'),
        role: 'assistant',
        content: errorMessage,
        createdAt: Date.now(),
        model: context.model,
        platform: context.platform,
        error: true
      }
      thread.lastRunError = assistantMessage.content
      thread.messages.push(assistantMessage)
    }
    if (activeThreadId.value === thread.id) {
      appStore.showError(errorMessage)
    } else {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
  } finally {
    const shouldFollow = activeThreadId.value === thread.id && isMessageScrollerNearBottom()
    if (runAbortControllers.get(thread.id)?.controller === controller) {
      runAbortControllers.delete(thread.id)
      thread.running = false
    }
    await scrollMessagesToBottomIfNeeded(shouldFollow)
  }
}

async function runStreamingChat(
  thread: PlaygroundThread,
  prompt: string,
  attachments: PlaygroundAttachment[],
  context: PlaygroundRunContext,
  controller: AbortController,
  runId: string
) {
  const runRequest = buildChatRunRequest(runId, prompt, attachments, thread.messages.slice(0, -1), context)
  const assistantMessage: PlaygroundMessage = {
    id: uid('msg'),
    role: 'assistant',
    content: '',
    createdAt: Date.now(),
    model: context.model,
    platform: context.platform,
    pending: true,
    progress: t('playground.streaming'),
    runId,
    runKeyId: context.keyId,
    runRequest: stripRunAPIKey(runRequest)
  }
  thread.messages.push(assistantMessage)
  await scrollMessagesToBottom()
  await writePlaygroundStateNow()

  await ensureBackendPlaygroundRun(runRequest, controller)
  const finalRun = await pollPlaygroundRun(runId, async (run) => {
    const shouldFollow = activeThreadId.value === thread.id && isMessageScrollerNearBottom()
    if (typeof run.content === 'string') {
      assistantMessage.content = run.content
    }
    assistantMessage.progress = run.status === 'queued' ? t('playground.waiting') : t('playground.streaming')
    thread.updatedAt = Date.now()
    await scrollMessagesToBottomIfNeeded(shouldFollow)
  }, controller)

  if (finalRun.status !== 'succeeded') {
    assistantMessage.pending = false
    assistantMessage.progress = ''
    assistantMessage.error = true
    assistantMessage.content = finalRun.error || t('playground.runFailed')
    thread.lastRunError = assistantMessage.content
    thread.updatedAt = Date.now()
    await writePlaygroundStateNow()
    throw markRunErrorHandled(buildRunError(finalRun), assistantMessage.content)
  }
  assistantMessage.content = assistantMessage.content || finalRun.content || t('playground.emptyTextResponse')
  assistantMessage.raw = finalRun.raw
  assistantMessage.pending = false
  assistantMessage.progress = ''
  assistantMessage.durationMs = finalRun.durationMs
  thread.updatedAt = Date.now()
  if (activeThreadId.value !== thread.id) {
    thread.unreadCount = (thread.unreadCount || 0) + 1
  }
  await writePlaygroundStateNow()
}

async function runImageGeneration(
  thread: PlaygroundThread,
  prompt: string,
  context: PlaygroundRunContext,
  imageConfig: PlaygroundImageConfig,
  controller: AbortController,
  runId: string
) {
  const runRequest = buildImageRunRequest(runId, prompt, context)
  const assistantMessage: PlaygroundMessage = {
    id: uid('msg'),
    role: 'assistant',
    content: '',
    createdAt: Date.now(),
    model: context.model,
    platform: context.platform,
    imageConfig,
    progress: t('playground.generatingImages'),
    pending: true,
    runId,
    runKeyId: context.keyId,
    runRequest: stripRunAPIKey(runRequest)
  }
  thread.messages.push(assistantMessage)
  await scrollMessagesToBottom()
  await writePlaygroundStateNow()

  try {
    await ensureBackendPlaygroundRun(runRequest, controller)
    const response = await pollPlaygroundRun(runId, async (run) => {
      assistantMessage.progress = run.status === 'queued' ? t('playground.waiting') : t('playground.generatingImages')
      thread.updatedAt = Date.now()
      await writePlaygroundStateNow()
    }, controller)
    if (response.status !== 'succeeded') {
      throw buildRunError(response)
    }

    assistantMessage.pending = false
    assistantMessage.progress = ''
    assistantMessage.durationMs = response.durationMs
    const images = response.images || []
    assistantMessage.content = images.length > 0 ? t('playground.imageGenerated') : t('playground.noImageReturned')
    assistantMessage.images = images.map((image, index) => ({
      ...image,
      storageId: uid(`image-${assistantMessage.id}-${index}`),
      mimeType: image.url.startsWith('data:')
        ? image.url.match(/^data:([^;,]+)/)?.[1]
        : undefined
    }))
    assistantMessage.raw = response.raw
    thread.updatedAt = Date.now()
    if (activeThreadId.value !== thread.id) {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
    await persistCompletedImageMessage(thread, assistantMessage)
  } catch (error) {
    if (controller.signal.aborted) throw error
    assistantMessage.pending = false
    assistantMessage.progress = ''
    assistantMessage.error = true
    assistantMessage.content = (error as Error)?.message || t('playground.runFailed')
    thread.lastRunError = assistantMessage.content
    thread.updatedAt = Date.now()
    if (activeThreadId.value !== thread.id) {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
    await persistCompletedImageMessage(thread, assistantMessage)
    throw markRunErrorHandled(error, assistantMessage.content)
  }
}

function stopRun() {
  const thread = activeThread.value
  if (!thread) return
  const handle = runAbortControllers.get(thread.id)
  handle?.controller.abort()
  if (handle?.runId) {
    void cancelPlaygroundRun(handle.runId).catch(() => undefined)
  }
  runAbortControllers.delete(thread.id)
  thread.running = false
  const pendingMessage = [...thread.messages].reverse().find((message) => message.pending)
  if (pendingMessage) {
    if (pendingMessage.runId) {
      void cancelPlaygroundRun(pendingMessage.runId).catch(() => undefined)
    }
    pendingMessage.pending = false
    pendingMessage.progress = ''
    if (!pendingMessage.content) pendingMessage.content = t('playground.requestStopped')
  }
}

function retryLastPrompt() {
  if (!lastPrompt.value || running.value) return
  draftPrompt.value = lastPrompt.value
  submitPrompt()
}

function readFile(file: File): Promise<PlaygroundAttachment> {
  return new Promise((resolve) => {
    const id = uid('file')
    const isImage = file.type.startsWith('image/')
    const isText = file.type.startsWith('text/') || /\.(md|txt|json|csv|log|xml|yaml|yml)$/i.test(file.name)
    const reader = new FileReader()

    reader.onload = () => {
      resolve({
        id,
        name: file.name,
        type: file.type,
        size: file.size,
        kind: isImage ? 'image' : isText ? 'text' : 'file',
        dataUrl: isImage ? String(reader.result || '') : undefined,
        text: isText ? String(reader.result || '').slice(0, 12000) : undefined
      })
    }

    if (isImage) {
      reader.readAsDataURL(file)
    } else if (isText) {
      reader.readAsText(file)
    } else {
      resolve({ id, name: file.name, type: file.type, size: file.size, kind: 'file' })
    }
  })
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  if (files.length === 0) return
  const attachments = await Promise.all(files.map(readFile))
  pendingAttachments.value = [...pendingAttachments.value, ...attachments]
  input.value = ''
}

function removeAttachment(id: string) {
  pendingAttachments.value = pendingAttachments.value.filter((attachment) => attachment.id !== id)
}

async function copyText(value: string) {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    appStore.showSuccess(t('common.copied'))
  } catch {
    appStore.showError(t('common.copyFailed'))
  }
}

watch(endpointOptions, (options) => {
  if (options.length === 0) return
  if (!options.some((option) => option.value === selectedEndpointBase.value)) {
    selectedEndpointBase.value = options[0].value
  }
}, { immediate: true })

watch(selectedKeyId, () => {
  if (restoringState) return
  models.value = []
  selectedModel.value = ''
  selectedModelsByMode.value = {}
  promptOptimizerModel.value = ''
  modelLoadError.value = ''
  lastRunError.value = ''
  loadModels()
})

watch(selectedEndpointBase, () => {
  if (restoringState) return
  models.value = []
  selectedModel.value = ''
  selectedModelsByMode.value = {}
  promptOptimizerModel.value = ''
  modelLoadError.value = ''
  lastRunError.value = ''
  loadModels()
})

watch(mode, () => {
  selectDefaultModel()
  selectDefaultPromptOptimizerModel()
})

watch(selectedModel, (value) => {
  if (restoringState) return
  if (!value.trim()) return
  rememberSelectedModelForMode()
})

watch(() => ({
  activeThreadId: activeThreadId.value,
  mode: mode.value,
  selectedKeyId: selectedKeyId.value,
  selectedEndpointBase: selectedEndpointBase.value,
  selectedModel: selectedModel.value,
  selectedModelsByMode: selectedModelsByMode.value,
  systemPrompt: systemPrompt.value,
  temperature: temperature.value,
  topP: topP.value,
  maxTokens: maxTokens.value,
  presencePenalty: presencePenalty.value,
  frequencyPenalty: frequencyPenalty.value,
  imageSizeMode: imageSizeMode.value,
  imageResolution: imageResolution.value,
  imageRatio: imageRatio.value,
  customImageWidth: customImageWidth.value,
  customImageHeight: customImageHeight.value,
  imageQuality: imageQuality.value,
  imageCount: imageCount.value,
  outputFormat: outputFormat.value,
  imagePromptStyle: imagePromptStyle.value,
  promptOptimizerModel: promptOptimizerModel.value,
  showComposerConfig: showComposerConfig.value,
  composerInputHeight: composerInputHeight.value,
  threads: threads.value
}), persistPlaygroundState, { deep: true })

watch([
  pendingAttachments,
  playgroundNotice,
  () => showComposerConfig.value,
  () => composerInputHeight.value,
  () => mode.value,
  () => selectedEndpointLabel.value
], () => {
  nextTick(updateComposerSpacer)
}, { deep: true })

watch(imagePreview, async (preview) => {
  imagePreviewResizeObserver?.disconnect()
  imagePreviewResizeObserver = null
  if (!preview) {
    imagePreviewViewportWidth.value = 0
    imagePreviewViewportHeight.value = 0
    return
  }
  await nextTick()
  updateImagePreviewViewportSize()
  if (imagePreviewViewport.value) {
    imagePreviewResizeObserver = new ResizeObserver(updateImagePreviewViewportSize)
    imagePreviewResizeObserver.observe(imagePreviewViewport.value)
  }
})

onMounted(async () => {
  playgroundViewMounted = true
  await appStore.fetchPublicSettings()
  await restorePlaygroundState()
  if (threads.value.length === 0) {
    createThread('chat')
  }
  await Promise.all([loadKeys(), loadAvailableChannels()])
  await loadModels()
  selectDefaultModel()
  selectDefaultPromptOptimizerModel()
  persistenceReady = true
  resumePendingPlaygroundRuns()
  await writePlaygroundStateNow()
  updateComposerSpacer()
  if (composerDock.value) {
    composerResizeObserver = new ResizeObserver(updateComposerSpacer)
    composerResizeObserver.observe(composerDock.value)
  }
  window.addEventListener(PLAYGROUND_STATE_UPDATED_EVENT, handlePlaygroundStateUpdated)
  scrollMessagesToBottom()
})

onBeforeUnmount(() => {
  playgroundViewMounted = false
  closeImagePreview()
  revokeAllImageObjectURLs()
  composerResizeObserver?.disconnect()
  composerResizeObserver = null
  imagePreviewResizeObserver?.disconnect()
  imagePreviewResizeObserver = null
  window.removeEventListener('pointermove', handleComposerInputResizePointerMove)
  window.removeEventListener('pointerup', handleComposerInputResizePointerUp)
  window.removeEventListener('pointercancel', handleComposerInputResizePointerUp)
  window.removeEventListener(PLAYGROUND_STATE_UPDATED_EVENT, handlePlaygroundStateUpdated)
  composerInputResizeState = null
  modelAbortController?.abort()
  promptOptimizeAbortController?.abort()
  for (const handle of runAbortControllers.values()) {
    handle.controller.abort()
  }
  for (const threadId of runAbortControllers.keys()) {
    runAbortControllers.delete(threadId)
  }
  void writePlaygroundStateNow()
})
</script>

<style scoped>
.image-option-button {
  display: inline-flex;
  min-height: 2rem;
  max-width: 100%;
  align-items: center;
  gap: 0.375rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(226 232 240);
  background: rgb(255 255 255);
  padding: 0.375rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(71 85 105);
  transition: border-color 150ms ease, background-color 150ms ease, color 150ms ease;
}

.image-option-button:hover {
  border-color: rgb(125 211 252);
  background: rgb(240 249 255);
  color: rgb(2 132 199);
}

.image-option-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.image-option-button:disabled:hover {
  border-color: rgb(226 232 240);
  background: rgb(255 255 255);
  color: rgb(71 85 105);
}

.image-option-button strong {
  font-weight: 700;
  color: rgb(37 99 235);
}

.dark .image-option-button {
  border-color: rgb(55 65 81);
  background: rgb(15 23 42);
  color: rgb(203 213 225);
}

.dark .image-option-button:hover {
  border-color: rgb(14 165 233);
  background: rgb(12 74 110 / 0.32);
  color: rgb(125 211 252);
}

.dark .image-option-button:disabled:hover {
  border-color: rgb(55 65 81);
  background: rgb(15 23 42);
  color: rgb(203 213 225);
}

.dark .image-option-button strong {
  color: rgb(147 197 253);
}

.playground-image-figure {
  width: min(22.5rem, calc(100vw - 7rem));
}

.playground-markdown {
  white-space: normal;
}

.playground-markdown :deep(p) {
  margin: 0.45rem 0;
}

.playground-markdown :deep(p:first-child),
.playground-markdown :deep(ul:first-child),
.playground-markdown :deep(ol:first-child),
.playground-markdown :deep(pre:first-child),
.playground-markdown :deep(blockquote:first-child) {
  margin-top: 0;
}

.playground-markdown :deep(p:last-child),
.playground-markdown :deep(ul:last-child),
.playground-markdown :deep(ol:last-child),
.playground-markdown :deep(pre:last-child),
.playground-markdown :deep(blockquote:last-child) {
  margin-bottom: 0;
}

.playground-markdown :deep(ul),
.playground-markdown :deep(ol) {
  margin: 0.5rem 0;
  padding-left: 1.25rem;
}

.playground-markdown :deep(ul) {
  list-style: disc;
}

.playground-markdown :deep(ol) {
  list-style: decimal;
}

.playground-markdown :deep(li) {
  margin: 0.2rem 0;
}

.playground-markdown :deep(a) {
  color: rgb(2 132 199);
  text-decoration: underline;
  text-underline-offset: 0.15em;
}

.playground-markdown :deep(blockquote) {
  margin: 0.65rem 0;
  border-left: 3px solid rgb(203 213 225);
  padding-left: 0.75rem;
  color: rgb(100 116 139);
}

.playground-markdown :deep(code) {
  border-radius: 0.375rem;
  background: rgb(241 245 249);
  padding: 0.1rem 0.35rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 0.88em;
}

.playground-markdown :deep(pre) {
  margin: 0.65rem 0;
  max-width: 100%;
  overflow-x: auto;
  border-radius: 0.5rem;
  background: rgb(15 23 42);
  padding: 0.875rem;
  color: rgb(226 232 240);
}

.playground-markdown :deep(pre code) {
  background: transparent;
  padding: 0;
  color: inherit;
}

.playground-markdown :deep(table) {
  margin: 0.65rem 0;
  width: 100%;
  border-collapse: collapse;
  font-size: 0.92em;
}

.playground-markdown :deep(th),
.playground-markdown :deep(td) {
  border: 1px solid rgb(226 232 240);
  padding: 0.35rem 0.5rem;
}

.playground-markdown :deep(th) {
  background: rgb(248 250 252);
  font-weight: 700;
}

.dark .playground-markdown :deep(a) {
  color: rgb(125 211 252);
}

.dark .playground-markdown :deep(blockquote) {
  border-left-color: rgb(71 85 105);
  color: rgb(148 163 184);
}

.dark .playground-markdown :deep(code) {
  background: rgb(30 41 59);
}

.dark .playground-markdown :deep(th),
.dark .playground-markdown :deep(td) {
  border-color: rgb(51 65 85);
}

.dark .playground-markdown :deep(th) {
  background: rgb(15 23 42);
}

.playground-select-value,
.playground-select-option {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.375rem;
}

.playground-select-value {
  width: 100%;
}

.playground-select-option {
  flex: 1;
}

.playground-platform-icon {
  display: inline-flex;
  height: 1.125rem;
  width: 1.125rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: rgb(248 250 252);
}

.dark .playground-platform-icon {
  background: rgb(15 23 42);
}

.playground-compact-select :deep(.select-trigger),
.image-option-select :deep(.select-trigger) {
  min-height: 2rem;
  border-radius: 0.5rem;
  padding: 0.375rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 600;
}

.playground-compact-select :deep(.select-trigger) {
  height: 2rem;
}

.playground-endpoint-select {
  width: min(max(var(--endpoint-select-width, 14ch), 9rem), 26rem);
  max-width: 100%;
}

:global(.select-dropdown-portal .playground-select-option) {
  gap: 0.375rem;
  font-size: 0.75rem;
  font-weight: 600;
}

:global(.select-dropdown-portal .playground-select-option .playground-platform-icon) {
  height: 1.125rem;
  width: 1.125rem;
}
</style>
