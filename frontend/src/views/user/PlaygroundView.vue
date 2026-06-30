<template>
  <AppLayout>
    <div class="-m-4 flex h-[calc(100vh-4rem)] min-h-[720px] overflow-hidden bg-slate-50 text-slate-900 dark:bg-dark-950 dark:text-white md:-m-6 lg:-m-8 lg:h-[calc(100vh-4rem)]">
      <aside class="hidden w-[320px] shrink-0 flex-col border-r border-slate-200 bg-slate-100/80 px-3 py-4 dark:border-dark-800 dark:bg-dark-900/80 lg:flex">
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
            <button
              type="button"
              class="inline-flex items-center gap-1.5 text-xs font-medium text-red-500 transition-colors hover:text-red-600"
              @click="clearHistory"
            >
              <Icon name="trash" size="xs" />
              {{ t('playground.clearHistory') }}
            </button>
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

      <section class="relative flex min-w-0 flex-1 flex-col bg-gradient-to-br from-slate-50 via-white to-slate-50 dark:from-dark-950 dark:via-dark-950 dark:to-dark-900">
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

        <main ref="messageScroller" class="min-h-0 flex-1 overflow-y-auto px-4 pb-52 pt-8 md:px-10 xl:px-16">
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
              class="flex"
              :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
            >
              <div v-if="message.role === 'assistant'" class="mr-4 mt-8 hidden h-8 w-8 shrink-0 items-center justify-center rounded-full bg-white text-sky-500 shadow-sm ring-1 ring-slate-200 dark:bg-dark-900 dark:text-sky-300 dark:ring-dark-700 md:flex">
                <Icon name="sparkles" size="sm" />
              </div>

              <div class="min-w-0" :class="message.role === 'user' ? 'max-w-[78%]' : 'max-w-[760px]'">
                <div
                  v-if="showAssistantDetails || message.role === 'user'"
                  class="mb-2 flex items-center gap-2 text-xs"
                  :class="message.role === 'user' ? 'justify-end text-slate-500' : 'text-slate-400'"
                >
                  <span class="font-semibold">
                    {{ message.role === 'user' ? userDisplayName : t('playground.assistant') }}
                  </span>
                  <span>{{ formatMessageTime(message.createdAt) }}</span>
                  <span
                    v-if="message.role === 'user'"
                    class="flex h-6 w-6 items-center justify-center rounded-full bg-blue-600 text-xs font-semibold text-white"
                  >
                    {{ currentMessages.length }}
                  </span>
                </div>

                <div
                  class="rounded-lg border px-4 py-3 shadow-sm"
                  :class="message.role === 'user'
                    ? 'border-sky-300 bg-sky-50 text-sky-700 dark:border-sky-700 dark:bg-sky-950/50 dark:text-sky-200'
                    : message.error
                      ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300'
                      : 'border-slate-200 bg-white text-slate-800 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-100'"
                >
                  <div v-if="message.content" class="whitespace-pre-wrap break-words text-sm leading-7">
                    {{ message.content }}
                  </div>
                  <div v-if="message.pending" class="mt-2 flex items-center gap-2 text-xs text-slate-400 dark:text-dark-400">
                    <span class="spinner h-3 w-3"></span>
                    {{ t('playground.streaming') }}
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

                  <div v-if="message.images?.length" class="mt-3 grid gap-3 sm:grid-cols-2">
                    <figure
                      v-for="(image, index) in message.images"
                      :key="`${image.url}-${index}`"
                      class="overflow-hidden rounded-lg border border-slate-200 bg-slate-50 dark:border-dark-700 dark:bg-dark-950"
                    >
                      <img :src="image.url" :alt="t('playground.generatedImageAlt', { n: index + 1 })" class="w-full object-contain" />
                      <figcaption v-if="image.revisedPrompt" class="border-t border-slate-200 p-3 text-xs leading-5 text-slate-500 dark:border-dark-700 dark:text-dark-400">
                        {{ image.revisedPrompt }}
                      </figcaption>
                    </figure>
                  </div>
                </div>

                <div v-if="message.role === 'assistant' && message.content" class="mt-3 flex items-center gap-2 text-slate-400">
                  <button class="rounded p-1 transition hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-800 dark:hover:text-white" :title="t('common.copy')" @click="copyText(message.content)">
                    <Icon name="copy" size="xs" />
                  </button>
                  <button class="rounded p-1 transition hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-800 dark:hover:text-white" :title="t('playground.retry')" @click="retryLastPrompt">
                    <Icon name="refresh" size="xs" />
                  </button>
                </div>
              </div>
            </article>
          </div>
        </main>

        <footer class="pointer-events-none absolute bottom-0 left-0 right-0 bg-gradient-to-t from-slate-50 via-slate-50 to-transparent px-4 pb-4 pt-10 dark:from-dark-950 dark:via-dark-950 md:px-10 xl:px-16">
          <div class="pointer-events-auto mx-auto max-w-[1220px] rounded-lg border border-slate-200 bg-slate-100/90 p-3 shadow-[0_20px_60px_-28px_rgba(15,23,42,0.35)] backdrop-blur dark:border-dark-700 dark:bg-dark-900/90">
            <div class="flex flex-wrap items-center gap-3 px-1 pb-3">
              <select v-model="selectedKeyId" class="control-select w-[180px]">
                <option value="">{{ t('playground.selectKey') }}</option>
                <option v-for="key in activeKeys" :key="key.id" :value="String(key.id)">
                  {{ key.name }}
                </option>
              </select>

              <select v-model="selectedEndpointBase" class="control-select min-w-[300px] max-w-[420px]">
                <option value="">{{ t('playground.selectEndpoint') }}</option>
                <option v-for="endpoint in endpointOptions" :key="endpoint.value" :value="endpoint.value">
                  {{ endpoint.displayLabel }}
                </option>
              </select>

              <select v-model="selectedModel" class="control-select min-w-[210px] max-w-[280px]">
                <option value="">{{ t('playground.selectModel') }}</option>
                <option v-for="model in visibleModels" :key="model.id" :value="model.id">
                  {{ model.label }}
                </option>
              </select>

              <label class="flex items-center gap-2 text-sm text-slate-500 dark:text-dark-300">
                <span>{{ t('playground.commonModel') }}</span>
                <input
                  v-model.trim="manualModel"
                  class="h-9 w-[190px] rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-sky-400 focus:ring-2 focus:ring-sky-100 dark:border-dark-700 dark:bg-dark-950 dark:text-white dark:focus:border-sky-500 dark:focus:ring-sky-900/40"
                  :placeholder="t('playground.manualModelCompact')"
                  @keydown.enter.prevent="useManualModel"
                />
              </label>

              <button
                type="button"
                class="ml-auto flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 transition hover:bg-white hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
                :title="t('playground.refreshModels')"
                :disabled="!selectedKey || loadingModels"
                @click="loadModels"
              >
                <Icon name="refresh" size="sm" :class="loadingModels ? 'animate-spin' : ''" />
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

            <div v-if="mode === 'image'" class="mb-2 flex flex-wrap items-center gap-2 px-1">
              <button
                type="button"
                class="image-option-button"
                @click="showImageSizeModal = true"
              >
                <Icon name="sparkles" size="xs" />
                <span>{{ t('playground.imageSizeButton') }}</span>
                <strong>{{ effectiveImageSize }}</strong>
              </button>

              <select v-model="imageQuality" class="image-option-select" :aria-label="t('playground.quality')">
                <option value="auto">{{ t('playground.quality') }}：{{ t('playground.qualityAuto') }}</option>
                <option value="low">{{ t('playground.quality') }}：{{ t('playground.qualityLow') }}</option>
                <option value="medium">{{ t('playground.quality') }}：{{ t('playground.qualityMedium') }}</option>
                <option value="high">{{ t('playground.quality') }}：{{ t('playground.qualityHigh') }}</option>
              </select>

              <select v-model="outputFormat" class="image-option-select" :aria-label="t('playground.outputFormat')">
                <option value="png">{{ t('playground.outputFormat') }}：PNG</option>
                <option value="webp">{{ t('playground.outputFormat') }}：WEBP</option>
                <option value="jpeg">{{ t('playground.outputFormat') }}：JPEG</option>
              </select>
            </div>

            <div class="flex min-h-[96px] items-stretch gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-950">
              <textarea
                v-model="draftPrompt"
                class="min-h-[74px] flex-1 resize-none bg-transparent text-sm leading-6 text-slate-900 outline-none placeholder:text-slate-400 dark:text-white"
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

            <p class="mt-2 px-1 text-xs text-slate-500 dark:text-dark-400">
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
                <select v-model="imageQuality" class="input">
                  <option value="auto">{{ t('playground.qualityAuto') }}</option>
                  <option value="low">{{ t('playground.qualityLow') }}</option>
                  <option value="medium">{{ t('playground.qualityMedium') }}</option>
                  <option value="high">{{ t('playground.qualityHigh') }}</option>
                </select>
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import {
  fetchModels,
  generateImage,
  resolvePlaygroundRequestBase,
  streamChatCompletion,
  type PlaygroundChatMessage,
  type PlaygroundImageResult,
  type PlaygroundModel
} from '@/api/playground'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { formatDateOnly, formatRelativeTime, formatTime } from '@/utils/format'
import type { ApiKey, GroupPlatform } from '@/types'

type PlaygroundMode = 'chat' | 'image' | 'video' | 'audio'
type MessageRole = 'user' | 'assistant'
type AttachmentKind = 'image' | 'text' | 'file'
type IconName = InstanceType<typeof Icon>['$props']['name']
type ImageSizeMode = 'auto' | 'ratio' | 'custom'
type ImageResolution = '1K' | '2K' | '4K'

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
  attachments?: PlaygroundAttachment[]
  images?: PlaygroundImageResult[]
  raw?: unknown
  pending?: boolean
  error?: boolean
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
  apiKey: string
  endpointBase?: string
  displayEndpoint?: string
  model: string
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

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const apiKeys = ref<ApiKey[]>([])
const selectedKeyId = ref('')
const selectedEndpointBase = ref('')
const loadingKeys = ref(false)
const loadingModels = ref(false)
const showSettings = ref(false)
const showAssistantDetails = ref(true)
const showImageSizeModal = ref(false)
const mode = ref<PlaygroundMode>('chat')
const models = ref<PlaygroundModel[]>([])
const selectedModel = ref('')
const manualModel = ref('')
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
const threads = ref<PlaygroundThread[]>([])
const activeThreadId = ref('')
const availableChannels = ref<UserAvailableChannel[]>([])
const historySearch = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const messageScroller = ref<HTMLElement | null>(null)
let modelAbortController: AbortController | null = null
const runAbortControllers = new Map<string, AbortController>()
let persistTimer: number | undefined
let restoringState = false
let persistenceReady = false

const PLAYGROUND_STORAGE_VERSION = 1
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
const effectiveModel = computed(() => manualModel.value.trim() || selectedModel.value.trim())
const effectiveImageSize = computed(() => {
  if (imageSizeMode.value === 'auto') return 'auto'
  if (imageSizeMode.value === 'custom') {
    return `${clampImageDimension(customImageWidth.value)}x${clampImageDimension(customImageHeight.value)}`
  }
  return calculateImageSize(imageResolution.value, imageRatio.value)
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
const selectedEndpointLabel = computed(() => {
  if (!selectedEndpoint.value) return ''
  return `${t('playground.endpoint')}: ${selectedEndpoint.value.value} (${t('playground.requestPath')}: ${selectedEndpoint.value.requestBase})`
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

const filteredThreads = computed(() => {
  const keyword = historySearch.value.trim().toLowerCase()
  const byMode = threads.value.filter((thread) => thread.mode === mode.value)
  if (!keyword) return byMode
  return byMode.filter((thread) => {
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

function normalizeThread(thread: Partial<PlaygroundThread>): PlaygroundThread {
  const normalized = createEmptyThread(thread.mode || 'chat')
  normalized.id = typeof thread.id === 'string' && thread.id ? thread.id : normalized.id
  normalized.title = typeof thread.title === 'string' && thread.title ? thread.title : normalized.title
  normalized.messages = Array.isArray(thread.messages)
    ? thread.messages
      .filter((message): message is PlaygroundMessage => Boolean(message && typeof message === 'object'))
      .map((message) => ({
        ...message,
        pending: false,
        content: message.pending && !message.content ? t('playground.requestInterrupted') : message.content
      }))
    : []
  normalized.createdAt = typeof thread.createdAt === 'number' ? thread.createdAt : normalized.createdAt
  normalized.updatedAt = typeof thread.updatedAt === 'number' ? thread.updatedAt : normalized.updatedAt
  normalized.draftPrompt = typeof thread.draftPrompt === 'string' ? thread.draftPrompt : ''
  normalized.pendingAttachments = Array.isArray(thread.pendingAttachments) ? thread.pendingAttachments : []
  normalized.lastPrompt = typeof thread.lastPrompt === 'string' ? thread.lastPrompt : ''
  normalized.unreadCount = typeof thread.unreadCount === 'number' ? thread.unreadCount : 0
  normalized.running = false
  normalized.lastRunError = typeof thread.lastRunError === 'string' ? thread.lastRunError : ''
  return normalized
}

function storageKey(): string {
  return `sub2api:playground:v${PLAYGROUND_STORAGE_VERSION}:${authStore.user?.id || authStore.user?.email || 'guest'}`
}

function writePlaygroundStateNow() {
  if (restoringState) return
  if (persistTimer !== undefined) {
    window.clearTimeout(persistTimer)
    persistTimer = undefined
  }
  const payload = {
    version: PLAYGROUND_STORAGE_VERSION,
    activeThreadId: activeThreadId.value,
    mode: mode.value,
    selectedKeyId: selectedKeyId.value,
    selectedEndpointBase: selectedEndpointBase.value,
    selectedModel: selectedModel.value,
    manualModel: manualModel.value,
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
    threads: threads.value.map((thread) => ({
      ...thread,
      running: false,
      messages: thread.messages.map((message) => ({
        ...message,
        pending: false,
        raw: undefined,
        content: message.pending && !message.content ? t('playground.requestInterrupted') : message.content
      }))
    }))
  }
  try {
    localStorage.setItem(storageKey(), JSON.stringify(payload))
  } catch (error) {
    console.warn('Failed to persist playground state:', error)
  }
}

function persistPlaygroundState() {
  if (restoringState || !persistenceReady) return
  if (persistTimer !== undefined) window.clearTimeout(persistTimer)
  persistTimer = window.setTimeout(() => {
    writePlaygroundStateNow()
  }, 180)
}

function restorePlaygroundState() {
  restoringState = true
  try {
    const raw = localStorage.getItem(storageKey())
    if (!raw) return
    const payload = JSON.parse(raw) as Record<string, unknown>
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
    manualModel.value = typeof payload.manualModel === 'string' ? payload.manualModel : manualModel.value
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
  } catch (error) {
    console.warn('Failed to restore playground state:', error)
  } finally {
    restoringState = false
  }
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

function selectMode(nextMode: PlaygroundMode) {
  mode.value = nextMode
  const existing = threads.value.find((thread) => thread.mode === nextMode)
  if (existing) {
    activeThreadId.value = existing.id
  } else {
    createThread(nextMode)
  }
  selectDefaultModel()
}

function selectDefaultModel() {
  if (manualModel.value.trim()) return
  const preferred = preferredModelForMode()
  if (selectedModel.value && modelAvailable(selectedModel.value)) {
    return
  }
  if (preferred) {
    selectedModel.value = preferred
    return
  }
  selectedModel.value = visibleModels.value[0]?.id || ''
}

function useManualModel() {
  if (manualModel.value.trim()) {
    selectedModel.value = ''
  }
}

function buildThreadTitle(prompt: string): string {
  const compact = prompt.replace(/\s+/g, ' ').trim()
  return compact ? compact.slice(0, 32) : modeLabel(mode.value)
}

async function scrollMessagesToBottom() {
  await nextTick()
  if (messageScroller.value) {
    messageScroller.value.scrollTop = messageScroller.value.scrollHeight
  }
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
  } catch (error) {
    if (controller.signal.aborted) return
    modelLoadError.value = (error as Error)?.message || t('playground.loadModelsFailed')
    appStore.showError(modelLoadError.value)
    selectDefaultModel()
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
    apiKey: selectedKey.value.key,
    endpointBase: selectedEndpoint.value?.requestBase,
    displayEndpoint: selectedEndpoint.value?.value,
    model: effectiveModel.value,
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

  const prompt = draftPrompt.value.trim()
  const attachments = [...pendingAttachments.value]
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
  runAbortControllers.set(thread.id, controller)
  thread.running = true
  await scrollMessagesToBottom()

  try {
    if (context.mode === 'image') {
      await runImageGeneration(thread, prompt, context, controller)
    } else {
      await runStreamingChat(thread, prompt, attachments, context, controller)
    }
    if (activeThreadId.value === thread.id) {
      appStore.showSuccess(t('playground.runSuccess'))
    }
  } catch (error) {
    if (controller.signal.aborted) return
    const assistantMessage: PlaygroundMessage = {
      id: uid('msg'),
      role: 'assistant',
      content: (error as Error)?.message || t('playground.runFailed'),
      createdAt: Date.now(),
      error: true
    }
    thread.lastRunError = assistantMessage.content
    thread.messages.push(assistantMessage)
    if (activeThreadId.value === thread.id) {
      appStore.showError(assistantMessage.content)
    } else {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
  } finally {
    if (runAbortControllers.get(thread.id) === controller) {
      runAbortControllers.delete(thread.id)
      thread.running = false
    }
    if (activeThreadId.value === thread.id) {
      await scrollMessagesToBottom()
    }
  }
}

async function runStreamingChat(
  thread: PlaygroundThread,
  prompt: string,
  attachments: PlaygroundAttachment[],
  context: PlaygroundRunContext,
  controller: AbortController
) {
  const assistantMessage: PlaygroundMessage = {
    id: uid('msg'),
    role: 'assistant',
    content: '',
    createdAt: Date.now(),
    pending: true
  }
  thread.messages.push(assistantMessage)
  await scrollMessagesToBottom()

  const response = await streamChatCompletion({
    apiKey: context.apiKey,
    endpointBase: context.endpointBase,
    displayEndpoint: context.displayEndpoint,
    model: context.model,
    messages: buildChatMessages(prompt, attachments, thread.messages.slice(0, -2), context.mode),
    temperature: context.temperature,
    topP: context.topP,
    maxTokens: context.maxTokens,
    presencePenalty: context.presencePenalty,
    frequencyPenalty: context.frequencyPenalty,
    signal: controller.signal,
    onDelta(delta) {
      assistantMessage.content += delta
      if (activeThreadId.value === thread.id) {
        scrollMessagesToBottom()
      }
    }
  })

  assistantMessage.content = assistantMessage.content || response.content || t('playground.emptyTextResponse')
  assistantMessage.raw = response.raw
  assistantMessage.pending = false
  thread.updatedAt = Date.now()
  if (activeThreadId.value !== thread.id) {
    thread.unreadCount = (thread.unreadCount || 0) + 1
  }
}

async function runImageGeneration(
  thread: PlaygroundThread,
  prompt: string,
  context: PlaygroundRunContext,
  controller: AbortController
) {
  const assistantMessage: PlaygroundMessage = {
    id: uid('msg'),
    role: 'assistant',
    content: t('playground.generatingImages'),
    createdAt: Date.now(),
    pending: true
  }
  thread.messages.push(assistantMessage)
  await scrollMessagesToBottom()

  const response = await generateImage({
    apiKey: context.apiKey,
    endpointBase: context.endpointBase,
    displayEndpoint: context.displayEndpoint,
    model: context.model,
    prompt,
    size: context.imageSize,
    n: context.imageCount,
    quality: context.imageQuality,
    outputFormat: context.outputFormat,
    signal: controller.signal
  })

  assistantMessage.pending = false
  assistantMessage.content = response.images.length > 0 ? t('playground.imageGenerated') : t('playground.noImageReturned')
  assistantMessage.images = response.images
  assistantMessage.raw = response.raw
  thread.updatedAt = Date.now()
  if (activeThreadId.value !== thread.id) {
    thread.unreadCount = (thread.unreadCount || 0) + 1
  }
}

function stopRun() {
  const thread = activeThread.value
  if (!thread) return
  runAbortControllers.get(thread.id)?.abort()
  runAbortControllers.delete(thread.id)
  thread.running = false
  const pendingMessage = [...thread.messages].reverse().find((message) => message.pending)
  if (pendingMessage) {
    pendingMessage.pending = false
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
  manualModel.value = ''
  modelLoadError.value = ''
  lastRunError.value = ''
  loadModels()
})

watch(selectedEndpointBase, () => {
  if (restoringState) return
  models.value = []
  selectedModel.value = ''
  manualModel.value = ''
  modelLoadError.value = ''
  lastRunError.value = ''
  loadModels()
})

watch(mode, () => {
  selectDefaultModel()
})

watch(manualModel, (value) => {
  if (value.trim()) {
    selectedModel.value = ''
  } else {
    selectDefaultModel()
  }
})

watch(() => ({
  activeThreadId: activeThreadId.value,
  mode: mode.value,
  selectedKeyId: selectedKeyId.value,
  selectedEndpointBase: selectedEndpointBase.value,
  selectedModel: selectedModel.value,
  manualModel: manualModel.value,
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
  threads: threads.value
}), persistPlaygroundState, { deep: true })

onMounted(async () => {
  await appStore.fetchPublicSettings()
  restorePlaygroundState()
  if (threads.value.length === 0) {
    createThread('chat')
  }
  await Promise.all([loadKeys(), loadAvailableChannels()])
  await loadModels()
  selectDefaultModel()
  persistenceReady = true
  writePlaygroundStateNow()
  scrollMessagesToBottom()
})

onBeforeUnmount(() => {
  modelAbortController?.abort()
  for (const controller of runAbortControllers.values()) {
    controller.abort()
  }
  runAbortControllers.clear()
  writePlaygroundStateNow()
})
</script>

<style scoped>
.control-select {
  height: 2.25rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(226 232 240);
  background: rgb(255 255 255);
  padding: 0 2rem 0 0.75rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(59 130 246);
  outline: none;
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.control-select:focus {
  border-color: rgb(56 189 248);
  box-shadow: 0 0 0 3px rgb(186 230 253 / 0.6);
}

.dark .control-select {
  border-color: rgb(55 65 81);
  background: rgb(15 23 42);
  color: rgb(147 197 253);
}

.dark .control-select:focus {
  border-color: rgb(14 165 233);
  box-shadow: 0 0 0 3px rgb(12 74 110 / 0.45);
}

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

.image-option-button strong {
  font-weight: 700;
  color: rgb(37 99 235);
}

.image-option-select {
  min-height: 2rem;
  max-width: 11rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(226 232 240);
  background: rgb(255 255 255);
  padding: 0.375rem 1.75rem 0.375rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(71 85 105);
  outline: none;
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.image-option-select:focus {
  border-color: rgb(56 189 248);
  box-shadow: 0 0 0 3px rgb(186 230 253 / 0.6);
}

.dark .image-option-button,
.dark .image-option-select {
  border-color: rgb(55 65 81);
  background: rgb(15 23 42);
  color: rgb(203 213 225);
}

.dark .image-option-button:hover {
  border-color: rgb(14 165 233);
  background: rgb(12 74 110 / 0.32);
  color: rgb(125 211 252);
}

.dark .image-option-button strong {
  color: rgb(147 197 253);
}

.dark .image-option-select:focus {
  border-color: rgb(14 165 233);
  box-shadow: 0 0 0 3px rgb(12 74 110 / 0.45);
}
</style>
