<template>
  <AppLayout>
    <div class="playground-mobile-shell -m-4 flex h-[calc(100svh-4rem)] min-h-0 overflow-hidden bg-gray-50 text-slate-900 dark:bg-dark-950 dark:text-white md:-m-6 md:h-[calc(100vh-4rem)] md:min-h-[100svh] lg:-m-8 lg:h-[calc(100vh-4rem)] lg:min-h-[720px]">
      <div
        v-if="mobileHistoryOpen"
        class="fixed inset-0 z-50 bg-black/45 backdrop-blur-[1px] lg:hidden"
        @click="mobileHistoryOpen = false"
      ></div>
      <aside
        class="fixed inset-y-0 left-0 z-50 flex w-[min(86vw,320px)] shrink-0 flex-col border-r border-slate-200 bg-white px-3 py-4 shadow-2xl transition-transform duration-200 ease-out dark:border-dark-800 dark:bg-dark-900 lg:static lg:z-auto lg:w-[320px] lg:translate-x-0 lg:bg-white/80 lg:shadow-none lg:dark:bg-dark-900/80"
        :class="mobileHistoryOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'"
      >
        <div class="flex h-10 items-center gap-3 px-1">
          <button
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-lg text-slate-500 transition-colors hover:bg-white hover:text-slate-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('common.back')"
            @click="mobileHistoryOpen = false"
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

      <section class="playground-mobile-workspace relative flex min-w-0 flex-1 flex-col overflow-hidden bg-gray-50 dark:bg-dark-950">
        <header class="playground-mobile-header relative z-30 flex h-16 shrink-0 items-center justify-between border-b border-slate-200/80 px-4 dark:border-dark-800 md:px-7">
          <div class="flex items-center gap-3">
            <button
              type="button"
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-white text-slate-600 shadow-sm ring-1 ring-slate-200 transition hover:bg-slate-50 hover:text-slate-900 dark:bg-dark-900 dark:text-dark-200 dark:ring-dark-700 dark:hover:bg-dark-800 lg:hidden"
              :title="t('playground.recentConversations')"
              @click="mobileHistoryOpen = true"
            >
              <Icon name="menu" size="sm" />
            </button>
            <button
              type="button"
              class="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-100 px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-200 dark:bg-dark-800 dark:text-dark-100 dark:hover:bg-dark-700 sm:px-4"
              @click="createThread(mode)"
            >
              <Icon name="chatBubble" size="sm" />
              <span class="hidden sm:inline">{{ t('playground.newConversation') }}</span>
            </button>
            <span class="hidden rounded-full bg-white px-3 py-1 text-xs font-medium text-slate-500 ring-1 ring-slate-200 dark:bg-dark-900 dark:text-dark-300 dark:ring-dark-700 md:inline-flex">
              {{ modeLabel(mode) }}
            </span>
            <div
              v-if="mode === 'image'"
              class="image-workspace-switch"
              role="group"
              :aria-label="t('playground.imageWorkspace')"
            >
              <button
                type="button"
                :class="imageWorkspaceMode === 'chat' ? 'is-active' : ''"
                :aria-pressed="imageWorkspaceMode === 'chat'"
                :title="t('playground.imageConversationMode')"
                @click="selectImageWorkspace('chat')"
              >
                <Icon name="chatBubble" size="sm" />
                <span class="hidden sm:inline">{{ t('playground.imageConversationMode') }}</span>
              </button>
              <button
                type="button"
                :class="imageWorkspaceMode === 'board' ? 'is-active' : ''"
                :aria-pressed="imageWorkspaceMode === 'board'"
                :title="t('playground.imageBoardMode')"
                @click="selectImageWorkspace('board')"
              >
                <Icon name="grid" size="sm" />
                <span class="hidden sm:inline">{{ t('playground.imageBoardMode') }}</span>
              </button>
            </div>
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
          class="playground-mobile-scroller min-h-0 flex-1 overflow-y-auto px-4 pt-8"
          :class="isBoardMode ? '' : 'md:px-10 xl:px-16'"
          :style="{ paddingBottom: `${composerSpacerHeight}px` }"
        >
          <div v-if="!selectedKey && !loadingKeys" class="flex h-full items-center justify-center text-center">
            <div class="max-w-sm">
              <Icon name="key" size="xl" class="mx-auto text-slate-300 dark:text-dark-600" />
              <p class="mt-3 text-sm font-semibold text-slate-700 dark:text-dark-100">{{ t('playground.noKeys') }}</p>
              <button type="button" class="mt-4 inline-flex h-10 items-center gap-2 rounded-lg bg-sky-500 px-4 text-sm font-semibold text-white transition hover:bg-sky-600" @click="openApiKeyManagement">
                <Icon name="key" size="sm" />
                {{ t('dashboard.createKey') }}
              </button>
            </div>
          </div>

          <div
            v-else-if="isImageBoardMode"
            ref="imageBoardRoot"
            class="image-board-root mx-auto max-w-[1280px]"
            @pointerdown="handleImageBoardPointerDown"
          >
            <div v-if="imageBoardTasks.length">
              <div v-if="selectedImageBoardTaskIds.size" class="image-board-selection-toolbar">
                <span>{{ t('playground.imageBoardSelected', { count: selectedImageBoardTaskIds.size }) }}</span>
                <div class="flex items-center gap-2">
                  <button type="button" :disabled="imageBoardBatchDownloading" @pointerdown.stop @click="clearImageBoardSelection">
                    {{ t('playground.imageBoardClearSelection') }}
                  </button>
                  <button type="button" :disabled="imageBoardBatchDownloading" @pointerdown.stop @click="downloadSelectedImageBoardTasks(false)">
                    <Icon name="download" size="xs" />
                    {{ t('playground.imageBoardDownloadSelected') }}
                  </button>
                  <button type="button" :disabled="imageBoardBatchDownloading" @pointerdown.stop @click="downloadSelectedImageBoardTasks(true)">
                    <span v-if="imageBoardBatchDownloading" class="spinner h-3.5 w-3.5"></span>
                    <Icon v-else name="download" size="xs" />
                    {{ t('playground.imageBoardDownloadZip') }}
                  </button>
                  <button type="button" class="is-danger" :disabled="imageBoardBatchDownloading" @pointerdown.stop @click="requestDeleteSelectedImageBoardTasks">
                    <Icon name="trash" size="xs" />
                    {{ t('playground.imageBoardDeleteSelected') }}
                  </button>
                </div>
              </div>
              <div class="image-board-grid">
                <article
                  v-for="task in paginatedImageBoardTasks"
                  :key="task.message.id"
                  class="image-board-card"
                  :class="[
                    task.message.error ? 'is-error' : '',
                    isImageBoardTaskSelected(task.message.id) ? 'is-selected' : ''
                  ]"
                  :data-image-board-task-id="task.message.id"
                  tabindex="0"
                  @click="handleImageBoardCardClick(task)"
                  @keydown.enter.prevent="handleImageBoardCardClick(task)"
                >
                  <span v-if="isImageBoardTaskSelected(task.message.id)" class="image-board-card-selected" aria-hidden="true">
                    <Icon name="check" size="xs" />
                  </span>
                  <div class="image-board-card-media">
                    <template v-if="task.message.images?.length">
                      <img
                        :src="task.message.images[0].url"
                        :alt="t('playground.generatedImageAlt', { n: 1 })"
                        class="h-full w-full object-cover"
                        loading="lazy"
                        decoding="async"
                        draggable="false"
                      >
                      <span
                        v-if="task.message.images.length > 1"
                        class="image-board-result-count"
                      >
                        {{ t('playground.imageResultCount', { count: task.message.images.length }) }}
                      </span>
                    </template>
                    <div v-else class="flex h-full flex-col items-center justify-center gap-2 px-6 text-center">
                      <span v-if="task.message.pending" class="spinner h-6 w-6"></span>
                      <Icon v-else-if="task.message.error" name="exclamationCircle" size="lg" class="text-red-400" />
                      <Icon v-else name="sparkles" size="lg" class="text-slate-300 dark:text-dark-600" />
                      <p class="text-xs leading-5" :class="task.message.error ? 'text-red-600 dark:text-red-300' : 'text-slate-500 dark:text-dark-300'">
                        {{ task.message.progress || task.message.content || t('playground.generatingImages') }}
                      </p>
                    </div>

                    <div class="image-board-card-overlay">
                      <div class="image-board-card-meta">
                        <span>{{ imageMessageSizeLabel(task.message, task.message.images?.[0]) }}</span>
                        <span v-if="task.message.model" :title="task.message.model">{{ task.message.model }}</span>
                      </div>
                      <div class="image-board-card-actions">
                        <button
                          v-if="task.message.images?.[0]"
                          type="button"
                          class="image-board-card-action"
                          :title="t('playground.redrawImage')"
                          :aria-label="t('playground.redrawImage')"
                          @pointerdown.stop
                          @click.stop="redrawImageBoardTask(task, 0)"
                        >
                          <Icon name="edit" size="xs" />
                        </button>
                        <button
                          type="button"
                          class="image-board-card-action"
                          :title="t('playground.reuseImageConfig')"
                          :aria-label="t('playground.reuseImageConfig')"
                          @pointerdown.stop
                          @click.stop="reuseImageBoardTask(task)"
                        >
                          <Icon name="refresh" size="xs" />
                        </button>
                        <button
                          type="button"
                          class="image-board-card-action"
                          :title="t('playground.imageBoardCopyPrompt')"
                          :aria-label="t('playground.imageBoardCopyPrompt')"
                          @pointerdown.stop
                          @click.stop="copyText(task.prompt)"
                        >
                          <Icon name="copy" size="xs" />
                        </button>
                        <button
                          type="button"
                          class="image-board-card-action is-danger"
                          :title="t('common.delete')"
                          :aria-label="t('common.delete')"
                          @pointerdown.stop
                          @click.stop="deleteImageBoardTask(task)"
                        >
                          <Icon name="trash" size="xs" />
                        </button>
                      </div>
                    </div>
                  </div>
                </article>
              </div>
              <div v-if="imageBoardSelection" class="image-board-selection-box" :style="imageBoardSelectionBoxStyle"></div>
              <nav v-if="imageBoardPageCount > 1" class="image-board-pagination" :aria-label="t('playground.imageBoardPagination')">
                <button type="button" :disabled="imageBoardPage === 1" :title="t('playground.imageBoardPreviousPage')" :aria-label="t('playground.imageBoardPreviousPage')" @click="selectImageBoardPage(imageBoardPage - 1)">
                  <Icon name="chevronLeft" size="sm" />
                </button>
                <template v-for="(item, index) in imageBoardPaginationItems" :key="`${item}-${index}`">
                  <span v-if="item === 'ellipsis'" aria-hidden="true">...</span>
                  <button v-else type="button" :class="item === imageBoardPage ? 'is-active' : ''" :aria-current="item === imageBoardPage ? 'page' : undefined" @click="selectImageBoardPage(item)">
                    {{ item }}
                  </button>
                </template>
                <button type="button" :disabled="imageBoardPage === imageBoardPageCount" :title="t('playground.imageBoardNextPage')" :aria-label="t('playground.imageBoardNextPage')" @click="selectImageBoardPage(imageBoardPage + 1)">
                  <Icon name="chevronRight" size="sm" />
                </button>
              </nav>
            </div>
            <div v-else class="flex h-full min-h-[360px] items-center justify-center text-center">
              <div class="max-w-md">
                <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-lg bg-sky-50 text-sky-500 dark:bg-sky-950/40 dark:text-sky-300">
                  <Icon name="grid" size="lg" />
                </div>
                <p class="mt-4 text-base font-semibold text-slate-800 dark:text-white">{{ t('playground.imageBoardEmpty') }}</p>
              </div>
            </div>
          </div>

          <div v-else-if="isVideoBoardMode" class="video-board-root mx-auto max-w-[1280px]">
            <div class="mb-4 flex min-w-0 flex-col gap-2 border-b border-slate-200 pb-3 dark:border-white/[0.08] sm:flex-row sm:items-center sm:justify-between">
              <div class="flex min-w-0 items-center gap-2 text-xs text-slate-500 dark:text-dark-300">
                <Icon name="folder" size="sm" class="shrink-0" />
                <span class="truncate" :title="videoDirectoryStatusLabel">{{ videoDirectoryStatusLabel }}</span>
              </div>
              <button
                type="button"
                class="inline-flex min-h-9 shrink-0 items-center justify-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:border-sky-300 hover:text-sky-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/[0.1] dark:bg-white/[0.04] dark:text-dark-100 dark:hover:border-sky-500/50 dark:hover:text-sky-300"
                :disabled="!supportsVideoDirectoryStorage"
                :title="videoDirectoryActionLabel"
                @click="chooseVideoStorageDirectory"
              >
                <Icon name="folder" size="sm" />
                {{ videoDirectoryActionLabel }}
              </button>
            </div>
            <div v-if="videoBoardTasks.length">
              <div class="video-board-grid">
                <article
                  v-for="task in paginatedVideoBoardTasks"
                  :key="task.message.id"
                  class="video-board-card"
                  :class="task.message.error ? 'is-error' : ''"
                  tabindex="0"
                  role="button"
                  :aria-label="t('playground.videoBoardOpenDetails')"
                  @mouseenter="playVideoBoardPreview"
                  @mouseleave="stopVideoBoardPreview"
                  @focusin="playVideoBoardPreview"
                  @focusout="stopVideoBoardPreview"
                  @click="openVideoBoardDetail(task, $event)"
                  @keydown.enter.prevent="openVideoBoardDetail(task, $event)"
                  @keydown.space.prevent="openVideoBoardDetail(task, $event)"
                >
                  <video
                    v-if="task.message.videos?.[0]?.url"
                    :src="task.message.videos[0].url"
                    class="video-board-card-media"
                    muted
                    loop
                    playsinline
                    preload="auto"
                    @loadedmetadata="primeVideoBoardPreview"
                  >
                    {{ t('playground.videoNotSupported') }}
                  </video>
                  <div v-else class="video-board-card-placeholder">
                      <span v-if="task.message.pending || task.message.videoDownloadProgress !== undefined" class="spinner h-7 w-7"></span>
                    <Icon v-else-if="task.message.error" name="exclamationCircle" size="lg" class="text-red-400" />
                    <Icon v-else name="play" size="lg" class="text-slate-300 dark:text-dark-600" />
                    <p :class="task.message.error ? 'text-red-500 dark:text-red-300' : ''">
                      {{ task.message.progress || task.message.content || t('playground.generatingVideo') }}
                    </p>
                    <div
                      v-if="task.message.videoDownloadProgress !== undefined"
                      class="video-download-progress"
                      role="progressbar"
                      :aria-label="t('playground.videoDownloading')"
                      aria-valuemin="0"
                      aria-valuemax="100"
                      :aria-valuenow="videoDownloadProgressPercent(task.message) ?? undefined"
                    >
                      <div class="video-download-progress-track">
                        <span
                          v-if="videoDownloadProgressPercent(task.message) !== null"
                          class="video-download-progress-fill"
                          :style="{ width: `${videoDownloadProgressPercent(task.message)}%` }"
                        ></span>
                        <span v-else class="video-download-progress-fill is-indeterminate"></span>
                      </div>
                      <span v-if="videoDownloadProgressPercent(task.message) !== null" class="video-download-progress-label">{{ Math.round(videoDownloadProgressPercent(task.message) || 0) }}%</span>
                    </div>
                  </div>
                  <div class="video-board-card-overlay">
                    <div class="video-board-card-meta">
                      <span>{{ videoBoardSizeLabel(task.message) }}</span>
                      <span v-if="task.message.model" :title="task.message.model">{{ task.message.model }}</span>
                    </div>
                    <div class="image-board-card-actions">
                      <button
                        type="button"
                        class="image-board-card-action"
                        :title="t('playground.imageBoardCopyPrompt')"
                        :aria-label="t('playground.imageBoardCopyPrompt')"
                        @click.stop="copyText(task.prompt)"
                      >
                        <Icon name="copy" size="xs" />
                      </button>
                      <button
                        type="button"
                        class="image-board-card-action is-danger"
                        :title="t('common.delete')"
                        :aria-label="t('common.delete')"
                        @click.stop="deleteVideoBoardTask(task)"
                      >
                        <Icon name="trash" size="xs" />
                      </button>
                    </div>
                  </div>
                </article>
              </div>
              <nav v-if="videoBoardPageCount > 1" class="image-board-pagination" :aria-label="t('playground.videoBoardPagination')">
                <button type="button" :disabled="videoBoardPage === 1" :title="t('playground.imageBoardPreviousPage')" :aria-label="t('playground.imageBoardPreviousPage')" @click="selectVideoBoardPage(videoBoardPage - 1)">
                  <Icon name="chevronLeft" size="sm" />
                </button>
                <template v-for="(item, index) in videoBoardPaginationItems" :key="`${item}-${index}`">
                  <span v-if="item === 'ellipsis'" aria-hidden="true">...</span>
                  <button v-else type="button" :class="item === videoBoardPage ? 'is-active' : ''" :aria-current="item === videoBoardPage ? 'page' : undefined" @click="selectVideoBoardPage(item)">
                    {{ item }}
                  </button>
                </template>
                <button type="button" :disabled="videoBoardPage === videoBoardPageCount" :title="t('playground.imageBoardNextPage')" :aria-label="t('playground.imageBoardNextPage')" @click="selectVideoBoardPage(videoBoardPage + 1)">
                  <Icon name="chevronRight" size="sm" />
                </button>
              </nav>
            </div>
            <div v-else class="flex h-full min-h-[360px] items-center justify-center text-center">
              <div class="max-w-md">
                <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-lg bg-sky-50 text-sky-500 dark:bg-sky-950/40 dark:text-sky-300">
                  <Icon name="play" size="lg" />
                </div>
                <p class="mt-4 text-base font-semibold text-slate-800 dark:text-white">{{ t('playground.videoBoardEmpty') }}</p>
              </div>
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
                class="w-fit min-w-0 max-w-full"
                :class="message.role === 'user'
                  ? 'max-w-[78%]'
                  : 'sm:max-w-[760px]'"
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
                  class="inline-block w-fit max-w-full rounded-lg border shadow-sm"
                  :class="[
                    message.images?.length ? 'p-2' : 'px-4 py-3',
                    message.role === 'user' ? 'ml-auto' : '',
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
                    <button
                      v-for="attachment in message.attachments"
                      :key="attachment.id"
                      type="button"
                      class="playground-attachment-chip"
                      :class="attachment.kind === 'image' && attachmentPreviewUrl(attachment) ? 'playground-attachment-chip-image' : ''"
                      :title="attachment.name"
                      @click="attachment.kind === 'image' && attachmentPreviewUrl(attachment) ? openAttachmentImagePreview(attachment) : undefined"
                    >
                      <img
                        v-if="attachment.kind === 'image' && attachmentPreviewUrl(attachment)"
                        :src="attachmentPreviewUrl(attachment)"
                        :alt="attachment.name"
                        class="h-full w-full object-cover"
                        loading="lazy"
                        decoding="async"
                      >
                      <template v-else>
                        <Icon name="document" size="xs" />
                        <span class="truncate">{{ attachment.name }}</span>
                      </template>
                    </button>
                  </div>

                  <div
                    v-if="message.images?.length"
                    class="playground-image-strip mt-3"
                  >
                    <figure
                      v-for="(image, index) in message.images"
                      :key="`${image.url}-${index}`"
                      class="playground-image-thumbnail group/thumbnail"
                    >
                      <div class="relative h-full w-full overflow-hidden rounded-lg">
                        <button
                          type="button"
                          class="absolute inset-0 block h-full w-full bg-slate-100 text-left dark:bg-dark-900"
                          :title="t('playground.viewOriginalImage')"
                          @click="openImagePreview(message, index)"
                        >
                          <img
                            :src="image.url"
                            :alt="t('playground.generatedImageAlt', { n: index + 1 })"
                            class="h-full w-full object-contain transition duration-200 group-hover/thumbnail:scale-[1.03]"
                            loading="lazy"
                            decoding="async"
                          />
                        </button>
                        <span class="pointer-events-none absolute left-1.5 top-1.5 rounded bg-black/55 px-1.5 py-0.5 text-[10px] font-semibold text-white backdrop-blur">
                          {{ index + 1 }}
                        </span>
                        <div class="playground-thumbnail-actions pointer-events-none absolute inset-0 flex items-center justify-center gap-2 bg-black/35 opacity-0 transition-opacity group-hover/thumbnail:opacity-100 group-focus-within/thumbnail:opacity-100">
                          <button
                            type="button"
                            class="pointer-events-auto inline-flex h-8 w-8 items-center justify-center rounded-full bg-white/95 text-slate-700 shadow-sm transition hover:text-sky-600"
                            :title="t('playground.redrawImage')"
                            :aria-label="t('playground.redrawImage')"
                            @click.stop="redrawGeneratedImage(message, image, index)"
                          >
                            <Icon name="edit" size="xs" />
                          </button>
                          <button
                            type="button"
                            class="pointer-events-auto inline-flex h-8 w-8 items-center justify-center rounded-full bg-white/95 text-slate-700 shadow-sm transition hover:text-sky-600"
                            :title="t('playground.viewOriginalImage')"
                            :aria-label="t('playground.viewOriginalImage')"
                            @click.stop="openImagePreview(message, index)"
                          >
                            <Icon name="eye" size="xs" />
                          </button>
                        </div>
                        <span class="pointer-events-none absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 bg-gradient-to-t from-black/75 to-transparent px-2 pb-1.5 pt-5 text-[10px] font-semibold text-white">
                          <span class="truncate">{{ imageMessageSizeLabel(message, image) }}</span>
                          <span class="shrink-0">{{ formatImageGenerationDuration(message.durationMs) }}</span>
                        </span>
                      </div>
                    </figure>
                  </div>

                  <div
                    v-if="message.videos?.length"
                    class="playground-image-strip mt-3"
                  >
                    <figure
                      v-for="(video, index) in message.videos"
                      :key="`${video.url}-${index}`"
                      class="playground-image-thumbnail group/thumbnail"
                    >
                      <div class="relative h-full w-full overflow-hidden rounded-lg">
                        <video
                          :src="video.url"
                          :poster="video.thumbnailUrl"
                          class="h-full w-full object-contain"
                          controls
                          preload="metadata"
                        >
                          {{ t('playground.videoNotSupported') }}
                        </video>
                        <span class="pointer-events-none absolute left-1.5 top-1.5 rounded bg-black/55 px-1.5 py-0.5 text-[10px] font-semibold text-white backdrop-blur">
                          {{ index + 1 }}
                        </span>
                      </div>
                    </figure>
                  </div>

                  <div v-if="firstImageDescription(message.images)" class="mt-2 max-w-[36rem]">
                    <button
                      type="button"
                      class="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 transition hover:text-sky-600 dark:text-dark-400 dark:hover:text-sky-300"
                      :aria-expanded="isImageDescriptionExpanded(message.id)"
                      :aria-controls="`image-description-${message.id}`"
                      @click="toggleImageDescription(message.id)"
                    >
                      <Icon :name="isImageDescriptionExpanded(message.id) ? 'chevronDown' : 'chevronRight'" size="xs" />
                      {{ t('playground.imageDescriptions') }}
                    </button>
                    <div
                      v-if="isImageDescriptionExpanded(message.id)"
                      :id="`image-description-${message.id}`"
                      class="mt-2 rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs leading-5 text-slate-600 dark:border-dark-700 dark:bg-dark-950 dark:text-dark-300"
                    >
                      <p>{{ firstImageDescription(message.images) }}</p>
                    </div>
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

        <footer ref="composerDock" class="playground-mobile-composer pointer-events-none absolute bottom-0 left-0 right-0 z-20 bg-gradient-to-t from-gray-50 via-gray-50 to-transparent px-4 pb-4 pt-10 dark:from-dark-950 dark:via-dark-950 md:px-10 xl:px-16">
          <div ref="composerShell" class="playground-composer-shell pointer-events-auto mx-auto max-w-[1220px] rounded-lg border border-slate-200 bg-white/95 p-3 shadow-[0_20px_60px_-28px_rgba(15,23,42,0.35)] backdrop-blur dark:border-dark-700 dark:bg-dark-900/95">
            <div v-if="activeComposerPanel" class="composer-control-panel">
              <section v-if="activeComposerPanel === 'model'" class="composer-model-picker" :aria-label="t('playground.model')">
                <template v-if="activeKeys.length">
                  <aside class="composer-model-key">
                    <p class="composer-panel-label">{{ t('playground.apiKey') }}</p>
                    <Select
                      v-model="selectedKeyId"
                      class="playground-compact-select playground-key-select"
                      :options="keySelectOptions"
                      :placeholder="t('playground.selectKey')"
                      searchable
                    >
                      <template #selected="{ option }">
                        <span v-if="option" class="playground-select-value playground-key-select-value">
                          <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                            <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                          </span>
                          <span class="playground-key-select-label min-w-0 break-words text-left leading-5">{{ option.label }}</span>
                        </span>
                        <span v-else>{{ t('playground.selectKey') }}</span>
                      </template>
                      <template #option="{ option, selected }">
                        <div class="playground-select-option playground-key-select-option">
                          <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                            <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                          </span>
                          <span class="min-w-0 flex-1 break-words text-left leading-4">{{ option.label }}</span>
                          <Icon v-if="selected" name="check" size="sm" class="text-primary-500" />
                        </div>
                      </template>
                    </Select>
                    <button
                      type="button"
                      class="composer-more-action mt-auto w-full justify-center"
                      :disabled="!selectedKey || loadingModels"
                      @click="loadModels"
                    >
                      <Icon name="refresh" size="sm" :class="loadingModels ? 'animate-spin' : ''" />
                      <span>{{ t('playground.refreshModels') }}</span>
                    </button>
                  </aside>
                  <div class="composer-model-results">
                  <div class="composer-model-search">
                    <Icon name="search" size="sm" />
                    <input
                      v-model="composerModelSearch"
                      type="search"
                      :placeholder="t('playground.searchModels')"
                    >
                  </div>
                  <div v-if="loadingModels" class="composer-model-empty">
                    <span class="spinner mx-auto mb-2 h-4 w-4"></span>
                    {{ t('playground.refreshModels') }}
                  </div>
                  <div v-else-if="filteredComposerModels.length" class="composer-model-list" role="listbox" :aria-label="t('playground.selectModel')">
                    <button
                      v-for="model in filteredComposerModels"
                      :key="model.id"
                      type="button"
                      class="composer-model-option"
                      :class="model.id === selectedModel ? 'is-selected' : ''"
                      :aria-selected="model.id === selectedModel"
                      role="option"
                      @click="selectComposerModel(model.id)"
                    >
                      <span class="playground-platform-icon" :class="platformIconClass(platformForModel(model.id, model.owned_by) || '')">
                        <PlatformIcon :platform="platformForModel(model.id, model.owned_by)" size="xs" />
                      </span>
                      <span class="min-w-0 flex-1 truncate text-left">{{ model.label || model.id }}</span>
                      <Icon v-if="model.id === selectedModel" name="check" size="sm" class="text-sky-600 dark:text-sky-300" />
                    </button>
                  </div>
                  <p v-else class="composer-model-empty">{{ modelLoadError || t('common.noOptionsFound') }}</p>
                </div>
                </template>
                <div v-else class="col-span-2 m-3 flex min-h-40 flex-col items-center justify-center text-center">
                  <Icon name="key" size="lg" class="text-slate-300 dark:text-dark-600" />
                  <p class="mt-2 text-sm font-semibold text-slate-700 dark:text-dark-100">{{ t('playground.noKeys') }}</p>
                  <button type="button" class="mt-3 inline-flex h-9 items-center gap-2 rounded-lg bg-sky-500 px-3 text-xs font-semibold text-white transition hover:bg-sky-600" @click="openApiKeyManagement">
                    <Icon name="key" size="xs" />
                    {{ t('dashboard.createKey') }}
                  </button>
                </div>
              </section>

              <section v-else-if="activeComposerPanel === 'image' && mode === 'image'" class="composer-image-panel" :aria-label="t('playground.composerImageGeneration')">
                <button
                  type="button"
                  class="image-option-button"
                  @click="showImageSizeModal = true"
                >
                  <Icon name="sparkles" size="xs" />
                  <span>{{ t('playground.imageSizeButton') }}</span>
                  <strong>{{ effectiveImageSize }}</strong>
                </button>

                <div class="image-count-control" role="group" :aria-label="t('playground.imageCount')">
                  <Icon name="grid" size="xs" />
                  <span>{{ t('playground.imageCount') }}</span>
                  <div class="image-count-segments">
                    <button
                      v-for="count in 4"
                      :key="count"
                      type="button"
                      :class="imageCount === count ? 'is-active' : ''"
                      :aria-pressed="imageCount === count"
                      @click="imageCount = count"
                    >
                      {{ count }}
                    </button>
                  </div>
                </div>

                <Select
                  v-model="imageQuality"
                  class="image-option-select"
                  :options="imageQualitySelectOptions"
                  :placeholder="t('playground.quality')"
                  :searchable="false"
                />

                <div class="image-format-control" role="group" :aria-label="t('playground.outputFormat')">
                  <span>{{ t('playground.outputFormat') }}</span>
                  <div class="image-format-segments">
                    <button
                      v-for="format in outputFormatSegmentOptions"
                      :key="format.value"
                      type="button"
                      :class="outputFormat === format.value ? 'is-active' : ''"
                      :aria-pressed="outputFormat === format.value"
                      @click="outputFormat = format.value"
                    >
                      {{ format.label }}
                    </button>
                  </div>
                </div>

                <button
                  type="button"
                  class="image-option-button"
                  :disabled="optimizingPrompt"
                  @click="openPromptOptimizerSettings"
                >
                  <Icon :name="optimizingPrompt ? 'refresh' : 'brain'" size="xs" :class="optimizingPrompt ? 'animate-spin' : ''" />
                  <span>{{ optimizingPrompt ? t('playground.optimizingPrompt') : t('playground.promptOptimizerSettings') }}</span>
                  <strong>{{ imagePromptStyleLabel() }}</strong>
                </button>
              </section>

              <section v-else-if="activeComposerPanel === 'video' && mode === 'video'" class="composer-image-panel composer-video-panel" :aria-label="t('playground.composerVideoGeneration')">
                <div class="video-option-field">
                  <span>{{ t('playground.videoDuration') }}</span>
                  <Select
                    v-model="videoDuration"
                    class="image-option-select"
                    :options="videoDurationSelectOptions"
                    :searchable="false"
                    :aria-label="t('playground.videoDuration')"
                  />
                </div>
                <div class="video-option-field">
                  <span>{{ t('playground.videoResolution') }}</span>
                  <Select
                    v-model="videoResolution"
                    class="image-option-select"
                    :options="videoResolutionSelectOptions"
                    :searchable="false"
                    :aria-label="t('playground.videoResolution')"
                  />
                </div>
                <div class="video-option-field">
                  <span>{{ t('playground.videoAspectRatio') }}</span>
                  <Select
                    v-model="videoAspectRatio"
                    class="image-option-select"
                    :options="videoAspectRatioSelectOptions"
                    :searchable="false"
                    :aria-label="t('playground.videoAspectRatio')"
                  />
                </div>
              </section>

            </div>

            <div v-if="pendingAttachments.length" class="mb-2 flex flex-wrap gap-2 px-1">
              <span
                v-for="attachment in pendingAttachments"
                :key="attachment.id"
                class="playground-pending-attachment"
                :class="attachment.kind === 'image' && attachmentPreviewUrl(attachment) ? 'playground-pending-attachment-image' : ''"
                :title="attachment.name"
              >
                <div v-if="attachment.kind === 'image' && attachmentPreviewUrl(attachment)" class="h-full w-full">
                  <img
                    :src="attachmentPreviewUrl(attachment)"
                    :alt="attachment.name"
                    class="h-full w-full object-cover"
                    loading="lazy"
                    decoding="async"
                  >
                  <button
                    type="button"
                    class="playground-attachment-edit"
                    :title="t('playground.editAttachmentImage')"
                    :aria-label="t('playground.editAttachmentImage')"
                    @click="openDoodleEditor(attachment)"
                  >
                    <Icon name="edit" size="sm" />
                  </button>
                </div>
                <template v-else>
                  <Icon name="document" size="xs" />
                  <span class="truncate">{{ attachment.name }}</span>
                </template>
                <button class="playground-attachment-remove" :title="t('common.delete')" @click="removeAttachment(attachment.id)">
                  <Icon name="x" size="xs" />
                </button>
              </span>
            </div>

            <div
              v-if="playgroundNotice"
              class="mb-3 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200"
            >
              <Icon name="infoCircle" size="sm" class="mt-0.5 shrink-0" />
              <span class="min-w-0 break-words">{{ playgroundNotice.message }}</span>
            </div>

            <div
              class="relative flex min-h-0 items-stretch gap-3 overflow-hidden rounded-lg border border-slate-200 bg-white px-4 pb-3 pt-7 transition-colors dark:border-dark-700 dark:bg-dark-950"
              :class="[
                composerInputResizing ? 'select-none' : '',
                composerDragActive ? 'border-sky-400 bg-sky-50/80 ring-2 ring-sky-200 dark:border-sky-500 dark:bg-sky-950/40 dark:ring-sky-900' : ''
              ]"
              :style="{ height: `${composerInputHeight}px` }"
              @dragenter.prevent="handleComposerDragEnter"
              @dragover.prevent="handleComposerDragOver"
              @dragleave.prevent="handleComposerDragLeave"
              @drop.prevent="handleComposerDrop"
            >
              <div v-if="composerDragActive" class="pointer-events-none absolute inset-0 z-30 flex items-center justify-center bg-white/90 text-sky-600 backdrop-blur-sm dark:bg-dark-950/90 dark:text-sky-300">
                <div class="flex items-center gap-2 text-sm font-semibold">
                  <Icon name="upload" size="md" />
                  <span>{{ mode === 'image' ? t('playground.dropImagesToUpload') : t('playground.dropFilesToUpload') }}</span>
                </div>
              </div>
              <button
                type="button"
                class="group absolute left-1/2 top-0 z-20 flex h-8 w-16 -translate-x-1/2 cursor-ns-resize touch-none items-center justify-center text-slate-300 transition-colors hover:text-sky-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300 dark:text-dark-500 dark:hover:text-sky-300 dark:focus-visible:ring-sky-800"
                :title="t('playground.resizeComposerInput')"
                @pointerdown.stop="handleComposerInputResizePointerDown"
              >
                <span class="h-1 w-7 rounded-full bg-current opacity-70 transition-all duration-150 group-hover:w-9 group-hover:opacity-100 group-focus-visible:w-9 group-focus-visible:opacity-100"></span>
              </button>
              <button
                v-if="mode === 'image'"
                type="button"
                class="absolute right-16 top-3 z-10 flex h-8 w-8 items-center justify-center rounded-lg text-sky-500 transition hover:bg-sky-50 hover:text-sky-600 disabled:cursor-not-allowed disabled:opacity-50 dark:text-sky-300 dark:hover:bg-sky-950/40 dark:hover:text-sky-200"
                :title="optimizingPrompt ? t('playground.optimizingPrompt') : t('playground.optimizePrompt')"
                :disabled="optimizingPrompt || running || !draftPrompt.trim() || !selectedPromptOptimizerKey || !promptOptimizerModel"
                @click="optimizeImagePrompt"
              >
                <Icon :name="optimizingPrompt ? 'refresh' : 'sparkles'" size="sm" :class="optimizingPrompt ? 'animate-spin' : ''" />
              </button>
              <textarea
                ref="composerTextarea"
                v-model="draftPrompt"
                class="h-full min-h-0 flex-1 resize-none overflow-y-auto bg-transparent pr-10 text-sm leading-6 text-slate-900 outline-none placeholder:text-slate-400 dark:text-white"
                :placeholder="composerPlaceholder"
                @keydown.enter.exact.prevent="submitPrompt"
                @paste="handleComposerPaste"
                @input="autoResizeComposerInput"
              ></textarea>

              <div class="flex shrink-0 items-end gap-2">
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
                  v-if="!running || mode === 'image'"
                  type="button"
                  class="flex h-10 w-10 items-center justify-center rounded-lg bg-sky-50 text-sky-500 transition hover:bg-sky-100 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-sky-950/40 dark:text-sky-300 dark:hover:bg-sky-900/50"
                  :title="t('playground.send')"
                  :disabled="mode !== 'image' && running"
                  @click="submitPrompt"
                >
                  <Icon name="arrowRight" size="md" />
                </button>
              </div>
            </div>

            <div class="composer-toolbar" role="toolbar" :aria-label="t('playground.composerOptions')">
              <input ref="fileInput" type="file" multiple class="hidden" :accept="mode === 'image' ? 'image/*' : undefined" @change="handleFileChange" />
              <button
                type="button"
                class="composer-toolbar-button composer-toolbar-icon-button"
                :title="t('playground.uploadFile')"
                :aria-label="t('playground.uploadFile')"
                @click="fileInput?.click()"
              >
                <Icon name="plus" size="md" />
              </button>
              <span class="composer-toolbar-divider" aria-hidden="true"></span>
              <button
                type="button"
                class="composer-toolbar-button composer-toolbar-model-button"
                :class="activeComposerPanel === 'model' ? 'is-active' : ''"
                :aria-expanded="activeComposerPanel === 'model'"
                aria-haspopup="listbox"
                @click="toggleComposerPanel('model')"
              >
                <Icon name="cpu" size="sm" />
                <span class="composer-toolbar-model-label min-w-0 flex-1 truncate">{{ selectedModel || t('playground.selectModel') }}</span>
                <Icon name="chevronDown" size="xs" />
              </button>
              <button
                v-if="mode === 'image'"
                type="button"
                class="composer-toolbar-button"
                :class="activeComposerPanel === 'image' ? 'is-active' : ''"
                :aria-expanded="activeComposerPanel === 'image'"
                aria-haspopup="dialog"
                @click="openImageComposerOptions"
              >
                <Icon name="sparkles" size="sm" />
                <span>{{ t('playground.composerImageGeneration') }}</span>
              </button>
              <button
                v-else-if="mode === 'video'"
                type="button"
                class="composer-toolbar-button"
                :class="activeComposerPanel === 'video' ? 'is-active' : ''"
                :aria-expanded="activeComposerPanel === 'video'"
                aria-haspopup="dialog"
                @click="openVideoComposerOptions"
              >
                <Icon name="play" size="sm" />
                <span>{{ t('playground.composerVideoGeneration') }}</span>
              </button>
            </div>

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

          <div v-if="mode === 'image'" class="border-t border-slate-100 pt-5 dark:border-dark-800">
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

          <div v-else-if="mode === 'video'" class="border-t border-slate-100 pt-5 dark:border-dark-800">
            <p class="mb-3 text-sm font-semibold text-slate-800 dark:text-white">{{ t('playground.composerVideoGeneration') }}</p>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <div>
                <label class="input-label">{{ t('playground.videoDuration') }}</label>
                <Select v-model="videoDuration" :options="videoDurationSelectOptions" :searchable="false" />
              </div>
              <div>
                <label class="input-label">{{ t('playground.videoResolution') }}</label>
                <Select v-model="videoResolution" :options="videoResolutionSelectOptions" :searchable="false" />
              </div>
              <div>
                <label class="input-label">{{ t('playground.videoAspectRatio') }}</label>
                <Select v-model="videoAspectRatio" :options="videoAspectRatioSelectOptions" :searchable="false" />
              </div>
            </div>
          </div>
        </div>
      </aside>
    </div>

    <div v-if="showImageSizeModal" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/30 p-4" @click.self="showImageSizeModal = false">
      <section class="max-h-[calc(100vh-2rem)] w-full max-w-[560px] overflow-y-auto rounded-2xl bg-white p-6 shadow-2xl dark:bg-dark-900">
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
              :disabled="!isImageResolutionSupported(resolution)"
              :title="isImageResolutionSupported(resolution) ? '' : t('playground.resolutionUnsupportedForRatio', { ratio: imageRatio, resolution })"
              class="h-12 rounded-xl border text-base font-medium transition"
              :class="!isImageResolutionSupported(resolution)
                ? 'cursor-not-allowed border-slate-200 bg-slate-50 text-slate-300 dark:border-dark-800 dark:bg-dark-900 dark:text-dark-600'
                : imageResolution === resolution
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
            <input v-model.number="customImageWidth" type="number" min="256" :max="currentImageMaxDimension" step="64" class="input" :placeholder="t('playground.width')" />
            <input v-model.number="customImageHeight" type="number" min="256" :max="currentImageMaxDimension" step="64" class="input" :placeholder="t('playground.height')" />
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

    <div v-if="showPromptOptimizerModal" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/30 p-4" @click.self="closePromptOptimizerSettings">
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
            @click="closePromptOptimizerSettings"
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
              :class="promptOptimizerDraftStyle === option.value
                ? 'border-sky-500 bg-sky-50 text-sky-600 dark:bg-sky-950/40'
                : 'border-slate-200 text-slate-600 hover:bg-slate-50 dark:border-dark-700 dark:text-dark-200 dark:hover:bg-dark-800'"
              @click="promptOptimizerDraftStyle = option.value as ImagePromptStyle"
            >
              {{ promptStyleOptionLabel(option) }}
            </button>
          </div>
        </div>

        <div class="mt-7">
          <label class="input-label">{{ t('playground.optimizerGroup') }}</label>
          <Select
            v-model="promptOptimizerDraftKeyId"
            :options="promptOptimizerGroupOptions"
            :placeholder="t('playground.selectOptimizerGroup')"
            searchable
          >
            <template #selected="{ option }">
              <span v-if="option" class="playground-select-value">
                <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                  <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                </span>
                <span class="truncate">{{ option.label }}</span>
              </span>
              <span v-else>{{ t('playground.selectOptimizerGroup') }}</span>
            </template>
            <template #option="{ option, selected }">
              <div class="playground-select-option">
                <span class="playground-platform-icon" :class="platformIconClass(optionPlatform(option) || '')">
                  <PlatformIcon :platform="optionPlatform(option)" size="xs" />
                </span>
                <span class="min-w-0 flex-1 text-left">
                  <span class="block truncate">{{ option.label }}</span>
                  <span v-if="option.description" class="block truncate text-xs text-slate-400 dark:text-dark-400">{{ option.description }}</span>
                </span>
                <Icon v-if="selected" name="check" size="sm" class="text-primary-500" />
              </div>
            </template>
          </Select>
        </div>

        <div class="mt-7">
          <label class="input-label">{{ t('playground.optimizerModel') }}</label>
          <Select
            v-model="promptOptimizerDraftModel"
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
          <p v-if="loadingPromptOptimizerModels" class="mt-2 text-xs text-slate-400 dark:text-dark-400">
            {{ t('playground.loadingOptimizerModels') }}
          </p>
          <p v-else-if="promptOptimizerModelLoadError" class="mt-2 text-xs text-red-500">
            {{ promptOptimizerModelLoadError }}
          </p>
        </div>

        <div class="mt-8 rounded-xl bg-slate-50 px-5 py-4 dark:bg-dark-950">
          <p class="text-sm font-semibold text-slate-400 dark:text-dark-300">{{ t('playground.willUse') }}</p>
          <p class="mt-2 truncate text-lg font-bold text-slate-800 dark:text-white">{{ imagePromptStyleLabel(promptOptimizerDraftStyle) }}</p>
          <p class="mt-1 truncate text-xs text-slate-500 dark:text-dark-300">{{ promptOptimizerGroupLabel(promptOptimizerDraftKeyId) }}</p>
          <p class="mt-1 truncate text-xs text-slate-400 dark:text-dark-400">{{ promptOptimizerDraftModel || t('playground.optimizerModel') }}</p>
        </div>

        <div class="mt-8 grid grid-cols-2 gap-3">
          <button type="button" class="h-12 rounded-xl bg-slate-100 text-sm font-semibold text-slate-600 transition hover:bg-slate-200 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700" @click="closePromptOptimizerSettings">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="h-12 rounded-xl bg-blue-500 text-sm font-semibold text-white transition hover:bg-blue-600 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="loadingPromptOptimizerModels || !promptOptimizerDraftKeyId || !promptOptimizerDraftModel"
            @click="savePromptOptimizerSettings"
          >
            {{ t('common.save') }}
          </button>
        </div>
      </section>
    </div>

    <ConfirmDialog
      :show="showImageBoardDeleteConfirm"
      :title="t('playground.imageBoardDeleteConfirmTitle')"
      :message="t('playground.imageBoardDeleteConfirmMessage', { count: selectedImageBoardTaskIds.size })"
      :confirm-text="t('common.delete')"
      danger
      @confirm="confirmDeleteSelectedImageBoardTasks"
      @cancel="showImageBoardDeleteConfirm = false"
    />

    <div v-if="imageBoardDetailTask" class="fixed inset-0 z-[75] flex items-center justify-center p-4" @click.self="closeImageBoardDetail()">
      <div class="absolute inset-0 bg-black/20 backdrop-blur-md dark:bg-black/40" @click="closeImageBoardDetail()"></div>
      <section class="relative z-10 flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-3xl border border-white/50 bg-white/90 shadow-[0_8px_40px_rgb(0,0,0,0.12)] ring-1 ring-black/5 backdrop-blur-xl dark:border-white/[0.08] dark:bg-dark-900/90 dark:shadow-[0_8px_40px_rgb(0,0,0,0.4)] dark:ring-white/10 md:flex-row" role="dialog" aria-modal="true" :aria-label="t('playground.imageBoardTaskDetails')">
        <div class="flex h-14 items-center justify-end px-4 md:hidden">
          <button type="button" class="rounded-full p-1 text-slate-400 transition hover:bg-slate-100 dark:text-dark-400 dark:hover:bg-white/[0.06]" :title="t('common.close')" :aria-label="t('common.close')" @click="closeImageBoardDetail()">
            <Icon name="x" size="lg" />
          </button>
        </div>
        <div class="relative flex h-64 min-h-[16rem] w-full shrink-0 items-center justify-center bg-slate-100 dark:bg-black/20 md:h-auto md:w-1/2">
          <template v-if="imageBoardDetailCurrentImage">
            <img
              :src="imageBoardDetailCurrentImage.url"
              :alt="t('playground.generatedImageAlt', { n: imageBoardDetailImageIndex + 1 })"
              class="max-h-[calc(100%-2rem)] max-w-[calc(100%-2rem)] cursor-pointer object-contain"
              loading="lazy"
              decoding="async"
              @click="openImageBoardDetailImage(imageBoardDetailImageIndex)"
            >
            <div class="pointer-events-none absolute left-4 top-4 flex items-center gap-1.5">
              <span v-if="imageBoardDetailRatioLabel" class="rounded-lg border border-sky-300/50 bg-sky-500/90 px-2 py-1 font-mono text-xs font-semibold text-white shadow-sm backdrop-blur-sm">{{ imageBoardDetailRatioLabel }}</span>
              <span class="rounded-lg border border-white/20 bg-slate-950/75 px-2 py-1 text-xs font-semibold text-white shadow-sm backdrop-blur-sm">{{ imageMessageSizeLabel(imageBoardDetailTask.message, imageBoardDetailCurrentImage) }}</span>
            </div>
            <button type="button" class="absolute right-4 top-4 flex h-8 w-8 items-center justify-center rounded-lg border border-white/70 bg-white/95 text-slate-700 shadow-sm transition hover:scale-105 hover:bg-sky-500 hover:text-white focus:outline-none focus:ring-2 focus:ring-sky-300 dark:border-white/20 dark:bg-dark-900/95 dark:text-dark-100 dark:hover:bg-sky-500 dark:focus:ring-sky-800" :title="t('playground.downloadImage')" :aria-label="t('playground.downloadImage')" @click="downloadImageBoardDetailImage()">
              <Icon name="download" size="sm" :stroke-width="2" />
            </button>
            <template v-if="imageBoardDetailImages.length > 1">
              <button type="button" class="absolute left-2 top-1/2 -translate-y-1/2 rounded-full bg-black/30 p-1.5 text-white transition hover:bg-black/50" :title="t('playground.previousImage')" :aria-label="t('playground.previousImage')" @click="navigateImageBoardDetail(-1)">
                <Icon name="chevronLeft" size="md" :stroke-width="2" />
              </button>
              <button type="button" class="absolute right-2 top-1/2 -translate-y-1/2 rounded-full bg-black/30 p-1.5 text-white transition hover:bg-black/50" :title="t('playground.nextImage')" :aria-label="t('playground.nextImage')" @click="navigateImageBoardDetail(1)">
                <Icon name="chevronRight" size="md" :stroke-width="2" />
              </button>
              <span class="pointer-events-none absolute bottom-2 left-1/2 -translate-x-1/2 rounded-full bg-black/50 px-2 py-0.5 text-xs text-white">{{ imageBoardDetailImageIndex + 1 }} / {{ imageBoardDetailImages.length }}</span>
            </template>
          </template>
          <div v-else class="w-full max-w-md px-4 text-center">
            <span v-if="imageBoardDetailTask.message.pending" class="spinner mx-auto h-10 w-10"></span>
            <Icon v-else-if="imageBoardDetailTask.message.error" name="exclamationCircle" size="xl" class="mx-auto text-red-400" />
            <Icon v-else name="sparkles" size="xl" class="mx-auto text-slate-300 dark:text-dark-600" />
            <p class="mt-3 text-sm leading-6 text-slate-500 dark:text-dark-300">{{ imageBoardDetailTask.message.progress || imageBoardDetailTask.message.content || t('playground.generatingImages') }}</p>
          </div>
        </div>

        <aside class="relative flex min-h-0 w-full flex-col overflow-hidden p-5 md:w-1/2">
          <button type="button" class="absolute right-3 top-3 hidden rounded-full p-1 text-slate-400 transition hover:bg-slate-100 dark:text-dark-400 dark:hover:bg-white/[0.06] md:flex" :title="t('common.close')" :aria-label="t('common.close')" @click="closeImageBoardDetail()">
            <Icon name="x" size="md" />
          </button>

          <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain pr-1">
            <div class="mb-2 flex items-center gap-1.5">
              <h3 class="text-xs font-medium uppercase tracking-wider text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardInput') }}</h3>
              <button type="button" class="rounded p-1 text-slate-400 transition hover:bg-slate-100 dark:text-dark-400 dark:hover:bg-white/[0.06]" :title="t('common.copy')" :aria-label="t('common.copy')" @click="copyText(imageBoardDetailTask.prompt)">
                <Icon name="copy" size="sm" />
              </button>
            </div>
            <p class="mb-4 max-h-40 overflow-y-auto whitespace-pre-wrap break-words pr-2 text-sm leading-relaxed text-slate-700 dark:text-dark-100">{{ imageBoardDetailTask.prompt || t('playground.attachmentOnlyPrompt') }}</p>

            <h3 class="mb-2 text-xs font-medium uppercase tracking-wider text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardParameterConfig') }}</h3>
            <div v-if="imageBoardDetailReferenceAttachments.length" class="mb-4">
              <h4 class="mb-2 text-xs font-medium text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardReferenceImages') }}</h4>
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="attachment in imageBoardDetailReferenceAttachments"
                  :key="attachment.id"
                  type="button"
                  class="group relative h-16 w-16 overflow-hidden rounded-lg border border-slate-200 bg-slate-100 shadow-sm transition hover:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-300 dark:border-dark-700 dark:bg-dark-950 dark:hover:border-sky-500 dark:focus:ring-sky-800"
                  :title="t('playground.viewOriginalImage')"
                  :aria-label="t('playground.viewOriginalImage')"
                  @click="openAttachmentImagePreview(attachment)"
                >
                  <img :src="attachmentPreviewUrl(attachment)" :alt="attachment.name" class="h-full w-full object-cover" loading="lazy" decoding="async">
                  <span class="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/35 text-white opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100">
                    <Icon name="eye" size="sm" />
                  </span>
                </button>
              </div>
            </div>
            <div class="mb-2 min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 text-xs dark:bg-white/[0.03]">
              <span class="text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardSource') }}</span>
              <div class="mt-0.5 truncate pr-2">
                <span class="font-medium text-slate-700 dark:text-dark-100">{{ imageBoardTaskSource(imageBoardDetailTask) }}</span>
                <span v-if="imageBoardDetailTask.message.model" class="text-slate-400 dark:text-dark-400"> · {{ imageBoardDetailTask.message.model }}</span>
              </div>
            </div>
            <div class="mb-4 grid min-w-0 grid-cols-2 gap-2 text-xs">
              <div class="min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 dark:bg-white/[0.03]">
                <span class="text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardSize') }}</span>
                <div class="mt-0.5 truncate pr-2 font-medium text-slate-700 dark:text-dark-100">{{ imageMessageSizeLabel(imageBoardDetailTask.message) }}</div>
              </div>
              <div class="min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 dark:bg-white/[0.03]">
                <span class="text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardQuality') }}</span>
                <div class="mt-0.5 truncate pr-2 font-medium text-slate-700 dark:text-dark-100">{{ imageBoardDetailTask.message.imageConfig?.quality || '-' }}</div>
              </div>
              <div class="min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 dark:bg-white/[0.03]">
                <span class="text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardFormat') }}</span>
                <div class="mt-0.5 truncate pr-2 font-medium text-slate-700 dark:text-dark-100">{{ imageBoardDetailTask.message.imageConfig?.outputFormat?.toUpperCase() || '-' }}</div>
              </div>
              <div class="min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 dark:bg-white/[0.03]">
                <span class="text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardCount') }}</span>
                <div class="mt-0.5 truncate pr-2 font-medium text-slate-700 dark:text-dark-100">{{ imageBoardDetailImages.length }}</div>
              </div>
            </div>
            <div class="mb-4 text-xs text-slate-400 dark:text-dark-400">
              <span>{{ t('playground.imageBoardCreatedAt', { time: imageBoardTaskDate(imageBoardDetailTask.message.createdAt) }) }}</span>
              <span> · {{ t('playground.imageBoardDuration') }} {{ formatImageGenerationDuration(imageBoardDetailTask.message.durationMs) }}</span>
            </div>
          </div>

          <div class="grid grid-cols-4 gap-2 border-t border-slate-100 pt-4 dark:border-white/[0.08] sm:flex">
            <button type="button" class="col-span-2 flex items-center justify-center gap-1.5 whitespace-nowrap rounded-xl bg-blue-50 px-3 py-2 text-sm font-medium text-blue-600 transition hover:bg-blue-100 dark:bg-blue-500/10 dark:text-blue-400 dark:hover:bg-blue-500/20 sm:flex-1" @click="reuseImageBoardTask(imageBoardDetailTask)">
              <Icon name="undo" size="sm" :stroke-width="2" />
              {{ t('playground.reuseImageConfig') }}
            </button>
            <button type="button" class="col-span-2 flex items-center justify-center gap-1.5 whitespace-nowrap rounded-xl bg-green-50 px-3 py-2 text-sm font-medium text-green-600 transition hover:bg-green-100 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-green-500/10 dark:text-green-400 dark:hover:bg-green-500/20 sm:flex-1" :disabled="!imageBoardDetailCurrentImage" @click="imageBoardDetailCurrentImage && redrawImageBoardTask(imageBoardDetailTask, imageBoardDetailImageIndex)">
              <Icon name="edit" size="sm" :stroke-width="2" />
              {{ t('playground.imageBoardEditOutput') }}
            </button>
            <button type="button" class="col-span-3 flex items-center justify-center gap-1.5 whitespace-nowrap rounded-xl bg-red-50 px-3 py-2 text-sm font-medium text-red-600 transition hover:bg-red-100 dark:bg-red-500/10 dark:text-red-400 dark:hover:bg-red-500/20 sm:flex-1" @click="deleteImageBoardTask(imageBoardDetailTask)">
              <Icon name="trash" size="sm" :stroke-width="2" />
              {{ t('playground.imageBoardDeleteTask') }}
            </button>
            <button type="button" class="col-span-1 flex w-full items-center justify-center rounded-xl transition sm:w-11" :class="imageBoardDetailTask.message.favorite ? 'bg-yellow-50 text-yellow-500 hover:bg-yellow-100 dark:bg-yellow-500/10 dark:hover:bg-yellow-500/20' : 'bg-slate-50 text-slate-400 hover:bg-yellow-50 hover:text-yellow-500 dark:bg-white/[0.04] dark:hover:bg-yellow-500/10'" :title="imageBoardDetailTask.message.favorite ? t('playground.imageBoardUnfavorite') : t('playground.imageBoardFavorite')" :aria-label="imageBoardDetailTask.message.favorite ? t('playground.imageBoardUnfavorite') : t('playground.imageBoardFavorite')" @click="toggleImageBoardTaskFavorite(imageBoardDetailTask)">
              <Icon name="star" size="md" :stroke-width="2" :class="imageBoardDetailTask.message.favorite ? 'fill-current' : ''" />
            </button>
          </div>
        </aside>
      </section>
    </div>

    <div v-if="videoBoardDetailTask" class="fixed inset-0 z-[75] flex items-center justify-center p-3 sm:p-4" @click.self="closeVideoBoardDetail()">
      <div class="absolute inset-0 bg-black/30 backdrop-blur-md dark:bg-black/55" @click="closeVideoBoardDetail()"></div>
      <section class="relative z-10 flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-white/50 bg-white/95 shadow-[0_8px_40px_rgb(0,0,0,0.16)] ring-1 ring-black/5 backdrop-blur-xl dark:border-white/[0.08] dark:bg-dark-900/95 dark:shadow-[0_8px_40px_rgb(0,0,0,0.45)] dark:ring-white/10 md:flex-row" role="dialog" aria-modal="true" :aria-label="t('playground.videoBoardTaskDetails')">
        <div class="flex h-12 shrink-0 items-center justify-between border-b border-slate-100 px-4 dark:border-white/[0.08] md:hidden">
          <h2 class="text-sm font-semibold text-slate-800 dark:text-white">{{ t('playground.videoBoardTaskDetails') }}</h2>
          <button type="button" class="flex h-9 w-9 items-center justify-center rounded-lg text-slate-400 transition hover:bg-slate-100 dark:text-dark-400 dark:hover:bg-white/[0.06]" :title="t('common.close')" :aria-label="t('common.close')" @click="closeVideoBoardDetail()">
            <Icon name="x" size="md" />
          </button>
        </div>

        <div class="relative flex min-h-[16rem] w-full shrink-0 items-center justify-center bg-black md:min-h-[34rem] md:w-[62%]">
          <video
            v-if="videoBoardDetailVideo?.url"
            :src="videoBoardDetailVideo.url"
            :poster="videoBoardDetailVideo.thumbnailUrl"
            class="max-h-[78vh] w-full bg-black object-contain"
            controls
            playsinline
            preload="metadata"
          >
            {{ t('playground.videoNotSupported') }}
          </video>
          <div v-else class="flex w-full max-w-md flex-col items-center justify-center px-6 text-center">
            <span v-if="videoBoardDetailTask.message.pending || videoBoardDetailTask.message.videoDownloadProgress !== undefined" class="spinner h-10 w-10"></span>
            <Icon v-else-if="videoBoardDetailTask.message.error" name="exclamationCircle" size="xl" class="text-red-400" />
            <Icon v-else name="play" size="xl" class="text-slate-600" />
            <p class="mt-4 text-sm leading-6" :class="videoBoardDetailTask.message.error ? 'text-red-300' : 'text-slate-300'">
              {{ videoBoardDetailTask.message.progress || videoBoardDetailTask.message.content || t('playground.generatingVideo') }}
            </p>
            <div
              v-if="videoBoardDetailTask.message.videoDownloadProgress !== undefined"
              class="video-download-progress mt-4"
              role="progressbar"
              :aria-label="t('playground.videoDownloading')"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-valuenow="videoDownloadProgressPercent(videoBoardDetailTask.message) ?? undefined"
            >
              <div class="video-download-progress-track">
                <span
                  v-if="videoDownloadProgressPercent(videoBoardDetailTask.message) !== null"
                  class="video-download-progress-fill"
                  :style="{ width: `${videoDownloadProgressPercent(videoBoardDetailTask.message)}%` }"
                ></span>
                <span v-else class="video-download-progress-fill is-indeterminate"></span>
              </div>
              <span v-if="videoDownloadProgressPercent(videoBoardDetailTask.message) !== null" class="video-download-progress-label">{{ Math.round(videoDownloadProgressPercent(videoBoardDetailTask.message) || 0) }}%</span>
            </div>
          </div>
        </div>

        <aside class="relative flex min-h-0 w-full flex-col overflow-hidden p-5 md:w-[38%]">
          <button type="button" class="absolute right-3 top-3 hidden h-9 w-9 items-center justify-center rounded-lg text-slate-400 transition hover:bg-slate-100 dark:text-dark-400 dark:hover:bg-white/[0.06] md:flex" :title="t('common.close')" :aria-label="t('common.close')" @click="closeVideoBoardDetail()">
            <Icon name="x" size="md" />
          </button>

          <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain pr-1">
            <div class="mb-2 flex items-center gap-1.5 pr-10">
              <h3 class="text-xs font-medium uppercase tracking-wider text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardInput') }}</h3>
              <button type="button" class="rounded p-1 text-slate-400 transition hover:bg-slate-100 dark:text-dark-400 dark:hover:bg-white/[0.06]" :title="t('common.copy')" :aria-label="t('common.copy')" @click="copyText(videoBoardDetailTask.prompt)">
                <Icon name="copy" size="sm" />
              </button>
            </div>
            <p class="mb-5 max-h-48 overflow-y-auto whitespace-pre-wrap break-words pr-2 text-sm leading-relaxed text-slate-700 dark:text-dark-100">{{ videoBoardDetailTask.prompt || t('playground.attachmentOnlyPrompt') }}</p>

            <h3 class="mb-2 text-xs font-medium uppercase tracking-wider text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardParameterConfig') }}</h3>
            <div class="mb-2 min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 text-xs dark:bg-white/[0.03]">
              <span class="text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardSource') }}</span>
              <div class="mt-0.5 truncate pr-2">
                <span class="font-medium text-slate-700 dark:text-dark-100">{{ videoBoardTaskSource(videoBoardDetailTask) }}</span>
                <span v-if="videoBoardDetailTask.message.model" class="text-slate-400 dark:text-dark-400"> · {{ videoBoardDetailTask.message.model }}</span>
              </div>
            </div>
            <div class="mb-4 grid min-w-0 grid-cols-2 gap-2 text-xs">
              <div class="min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 dark:bg-white/[0.03]">
                <span class="text-slate-400 dark:text-dark-400">{{ t('playground.videoResolution') }}</span>
                <div class="mt-0.5 truncate font-medium text-slate-700 dark:text-dark-100">{{ videoBoardSizeLabel(videoBoardDetailTask.message) }}</div>
              </div>
              <div class="min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 dark:bg-white/[0.03]">
                <span class="text-slate-400 dark:text-dark-400">{{ t('playground.videoAspectRatio') }}</span>
                <div class="mt-0.5 truncate font-medium text-slate-700 dark:text-dark-100">{{ videoBoardDetailTask.message.runRequest?.aspectRatio || '-' }}</div>
              </div>
              <div class="min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 dark:bg-white/[0.03]">
                <span class="text-slate-400 dark:text-dark-400">{{ t('playground.videoDuration') }}</span>
                <div class="mt-0.5 truncate font-medium text-slate-700 dark:text-dark-100">{{ videoBoardDurationLabel(videoBoardDetailTask.message) }}</div>
              </div>
              <div class="min-w-0 overflow-hidden rounded-lg bg-slate-50 px-3 py-2 dark:bg-white/[0.03]">
                <span class="text-slate-400 dark:text-dark-400">{{ t('playground.imageBoardDuration') }}</span>
                <div class="mt-0.5 truncate font-medium text-slate-700 dark:text-dark-100">{{ formatImageGenerationDuration(videoBoardDetailTask.message.durationMs) }}</div>
              </div>
            </div>
            <div class="mb-4 text-xs text-slate-400 dark:text-dark-400">
              {{ t('playground.imageBoardCreatedAt', { time: imageBoardTaskDate(videoBoardDetailTask.message.createdAt) }) }}
            </div>
          </div>

          <div class="grid grid-cols-2 gap-2 border-t border-slate-100 pt-4 dark:border-white/[0.08]">
            <button type="button" class="flex min-h-11 items-center justify-center gap-1.5 rounded-lg bg-sky-50 px-3 py-2 text-sm font-medium text-sky-700 transition hover:bg-sky-100 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-sky-500/10 dark:text-sky-300 dark:hover:bg-sky-500/20" :disabled="!videoBoardDetailVideo?.url" @click="downloadVideoBoardDetail()">
              <Icon name="download" size="sm" />
              {{ t('playground.downloadVideo') }}
            </button>
            <button type="button" class="flex min-h-11 items-center justify-center gap-1.5 rounded-lg bg-slate-50 px-3 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-100 dark:bg-white/[0.04] dark:text-dark-100 dark:hover:bg-white/[0.08]" @click="copyText(videoBoardDetailTask.prompt)">
              <Icon name="copy" size="sm" />
              {{ t('playground.imageBoardCopyPrompt') }}
            </button>
            <button type="button" class="col-span-2 flex min-h-11 items-center justify-center gap-1.5 rounded-lg bg-red-50 px-3 py-2 text-sm font-medium text-red-600 transition hover:bg-red-100 dark:bg-red-500/10 dark:text-red-400 dark:hover:bg-red-500/20" @click="deleteVideoBoardDetailTask()">
              <Icon name="trash" size="sm" />
              {{ t('playground.imageBoardDeleteTask') }}
            </button>
          </div>
        </aside>
      </section>
    </div>

    <div v-if="doodleEditor" class="fixed inset-0 z-[80] flex items-center justify-center bg-black/60" :class="doodleEditorFullscreen ? 'p-0' : 'p-2 sm:p-4'" @click.self="closeDoodleEditor()">
      <section
        class="flex w-full flex-col overflow-hidden bg-white shadow-2xl dark:bg-dark-900"
        :class="doodleEditorFullscreen ? 'h-[100dvh] max-w-none rounded-none border-0' : 'h-[min(92vh,900px)] max-w-6xl rounded-lg border border-slate-200 dark:border-dark-700'"
        :aria-busy="doodleSaving"
      >
        <header class="flex items-center justify-between border-b border-slate-200 px-4 py-3 dark:border-dark-700">
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-slate-900 dark:text-white">{{ t('playground.doodleEditor') }}</h2>
            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-dark-400">{{ doodleEditor.name }}</p>
          </div>
          <div class="flex items-center gap-1">
            <button
              type="button"
              class="flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 disabled:cursor-wait disabled:opacity-50 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
              :title="doodleEditorFullscreen ? t('playground.doodleExitFullscreen') : t('playground.doodleEnterFullscreen')"
              :aria-label="doodleEditorFullscreen ? t('playground.doodleExitFullscreen') : t('playground.doodleEnterFullscreen')"
              :aria-pressed="doodleEditorFullscreen"
              :disabled="doodleSaving"
              @click="toggleDoodleEditorFullscreen"
            >
              <Icon :name="doodleEditorFullscreen ? 'shrink' : 'expand'" size="md" />
            </button>
            <button type="button" class="flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 disabled:cursor-wait disabled:opacity-50 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white" :title="t('common.close')" :disabled="doodleSaving" @click="closeDoodleEditor()">
              <Icon name="x" size="md" />
            </button>
          </div>
        </header>

        <div class="flex flex-wrap items-center gap-2 border-b border-slate-200 px-3 py-2 dark:border-dark-700">
          <div class="inline-flex overflow-hidden rounded-lg border border-slate-200 dark:border-dark-700">
            <button type="button" class="doodle-tool-button" :class="doodleTool === 'brush' ? 'is-active' : ''" :title="t('playground.doodleBrush')" :disabled="doodleSaving" @click="doodleTool = 'brush'">
              <Icon name="edit" size="sm" />
            </button>
            <button type="button" class="doodle-tool-button" :class="doodleTool === 'eraser' ? 'is-active' : ''" :title="t('playground.doodleEraser')" :disabled="doodleSaving" @click="doodleTool = 'eraser'">
              <Icon name="eraser" size="sm" />
            </button>
            <button type="button" class="doodle-tool-button" :class="doodleTool === 'sticker' ? 'is-active' : ''" :title="t('playground.doodleStickers')" :disabled="doodleSaving" @click="toggleDoodleStickerPanel">
              <Icon name="sparkles" size="sm" />
            </button>
            <button type="button" class="doodle-tool-button" :class="doodleTool === 'pan' ? 'is-active' : ''" :title="t('playground.doodlePan')" :disabled="doodleSaving" @click="doodleTool = 'pan'">
              <Icon name="move" size="sm" />
            </button>
          </div>

          <div class="flex items-center gap-1.5" :aria-label="t('playground.doodleColor')">
            <button
              v-for="color in doodleColors"
              :key="color"
              type="button"
              class="h-6 w-6 rounded-full border-2 shadow-sm transition hover:scale-110 disabled:cursor-wait disabled:opacity-50"
              :class="doodleColor === color ? 'border-sky-500 ring-2 ring-sky-200 dark:ring-sky-900' : 'border-white dark:border-dark-600'"
              :style="{ backgroundColor: color }"
              :title="color"
              :aria-label="color"
              :aria-pressed="doodleColor === color"
              :disabled="doodleSaving"
              @click="selectDoodleColor(color)"
            ></button>
          </div>

          <label class="flex min-w-[150px] flex-1 items-center gap-2 text-xs font-medium text-slate-500 dark:text-dark-300">
            <span class="shrink-0">{{ t('playground.doodleSize') }}</span>
            <input v-model.number="doodleBrushSize" type="range" min="2" max="48" step="1" class="min-w-[80px] flex-1 accent-sky-500 disabled:cursor-wait disabled:opacity-50" :disabled="doodleSaving">
            <span class="w-7 text-right tabular-nums">{{ doodleBrushSize }}</span>
          </label>

          <label class="flex min-w-[160px] flex-1 items-center gap-2 text-xs font-medium text-slate-500 dark:text-dark-300">
            <span class="shrink-0">{{ t('playground.doodleOpacity') }}</span>
            <input v-model.number="doodleBrushOpacity" type="range" min="0.1" max="1" step="0.05" class="min-w-[80px] flex-1 accent-sky-500 disabled:cursor-wait disabled:opacity-50" :disabled="doodleSaving || doodleTool === 'eraser' || doodleTool === 'sticker'">
            <span class="w-9 text-right tabular-nums">{{ Math.round(doodleBrushOpacity * 100) }}%</span>
          </label>

          <div class="inline-flex items-center overflow-hidden rounded-lg border border-slate-200 dark:border-dark-700" :aria-label="t('playground.doodleZoom')">
            <button type="button" class="doodle-tool-button" :title="t('playground.doodleZoomOut')" :disabled="doodleSaving || doodleZoom <= 0.02" @click="adjustDoodleZoom(-0.25)">
              <Icon name="minus" size="sm" />
            </button>
            <span class="min-w-12 px-1 text-center text-xs font-medium tabular-nums text-slate-500 dark:text-dark-300">{{ Math.round(doodleZoom * 100) }}%</span>
            <button type="button" class="doodle-tool-button" :title="t('playground.doodleZoomIn')" :disabled="doodleSaving" @click="adjustDoodleZoom(0.25)">
              <Icon name="plus" size="sm" />
            </button>
          </div>

          <div class="ml-auto flex items-center gap-1">
            <button type="button" class="doodle-action-button" :disabled="doodleSaving || doodleStrokes.length === 0" :title="t('playground.doodleUndo')" @click="undoDoodleStroke">
              <Icon name="undo" size="sm" />
            </button>
            <button type="button" class="doodle-action-button" :disabled="doodleSaving || doodleRedoStrokes.length === 0" :title="t('playground.doodleRedo')" @click="redoDoodleStroke">
              <Icon name="redo" size="sm" />
            </button>
            <button type="button" class="doodle-action-button hover:!text-red-500" :disabled="doodleSaving || doodleStrokes.length === 0" :title="t('playground.doodleClear')" @click="clearDoodleStrokes">
              <Icon name="trash" size="sm" />
            </button>
            <button type="button" class="doodle-action-button hover:!text-red-500" :disabled="doodleSaving || !selectedDoodleStickerId" :title="t('playground.doodleDeleteSticker')" @click="deleteSelectedDoodleSticker">
              <Icon name="x" size="sm" />
            </button>
          </div>
        </div>

        <div v-if="doodleStickerPanelOpen" class="doodle-sticker-panel border-b border-slate-200 px-3 py-2 dark:border-dark-700">
          <p class="mb-1.5 text-xs font-medium text-slate-500 dark:text-dark-300">{{ t('playground.doodleStickers') }}</p>
          <div class="flex flex-wrap gap-1">
            <button
              v-for="sticker in doodleStickers"
              :key="sticker"
              type="button"
              class="doodle-sticker-button"
              :class="doodleSelectedSticker === sticker ? 'is-active' : ''"
              :title="sticker"
              :aria-label="sticker"
              :aria-pressed="doodleSelectedSticker === sticker"
              :disabled="doodleSaving"
              @click="selectDoodleSticker(sticker)"
            >
              {{ sticker }}
            </button>
          </div>
        </div>

        <div ref="doodleCanvasViewport" class="doodle-canvas-stage relative min-h-0 flex-1 touch-none overflow-hidden p-3 sm:p-5">
          <canvas
            ref="doodleCanvas"
            class="pointer-events-auto absolute left-1/2 top-1/2 block max-w-none touch-none rounded border border-slate-300 bg-white shadow-sm will-change-transform dark:border-dark-600"
            :class="[doodleCanvasPanning ? 'transition-none' : 'transition-transform duration-100', doodleSaving ? 'pointer-events-none cursor-wait' : doodleCanvasCursorClass]"
            :style="{ width: `${doodleCanvasBaseSize.width}px`, height: `${doodleCanvasBaseSize.height}px`, transform: `translate(calc(-50% + ${doodlePanX}px), calc(-50% + ${doodlePanY}px)) scale(${doodleZoom})` }"
            :aria-label="t('playground.doodleCanvas')"
            @pointerdown="handleDoodlePointerDown"
            @pointermove="handleDoodlePointerMove"
            @pointerup="handleDoodlePointerUp"
            @pointercancel="handleDoodlePointerUp"
            @wheel.prevent="handleDoodleCanvasWheel"
            @dblclick="fitDoodleCanvas"
          ></canvas>
        </div>

        <footer class="flex justify-end gap-2 border-t border-slate-200 px-4 py-3 dark:border-dark-700">
          <button type="button" class="h-10 rounded-lg border border-slate-200 px-4 text-sm font-semibold text-slate-600 transition hover:bg-slate-50 disabled:cursor-wait disabled:opacity-50 dark:border-dark-700 dark:text-dark-200 dark:hover:bg-dark-800" :disabled="doodleSaving" @click="closeDoodleEditor()">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="inline-flex h-10 items-center gap-2 rounded-lg bg-sky-500 px-4 text-sm font-semibold text-white transition hover:bg-sky-600 disabled:cursor-not-allowed disabled:opacity-60" :disabled="doodleSaving" @click="saveDoodleAttachment">
            <Icon :name="doodleSaving ? 'refresh' : 'check'" size="sm" :class="doodleSaving ? 'animate-spin' : ''" />
            {{ t('playground.doodleApply') }}
          </button>
        </footer>
      </section>
    </div>

    <div
      v-if="imagePreview"
      ref="imagePreviewViewport"
      class="fixed inset-0 z-[90] touch-none select-none overflow-hidden bg-black/90 focus:outline-none"
      :class="imagePreviewCursorClass"
      role="dialog"
      aria-modal="true"
      :aria-label="t('playground.imagePreviewTitle')"
      tabindex="-1"
      @click="handleImagePreviewBackdropClick"
      @wheel.prevent="handleImagePreviewWheel"
      @pointerdown="handleImagePreviewPointerDown"
      @pointermove="handleImagePreviewPointerMove"
      @pointerup="handleImagePreviewPointerUp"
      @pointercancel="handleImagePreviewPointerUp"
      @dblclick="resetImagePreviewView"
    >
      <div class="absolute right-4 top-4 z-10 flex items-center gap-2" @pointerdown.stop>
        <span v-if="imagePreview.total && imagePreview.total > 1" class="rounded-full bg-black/45 px-2.5 py-1 text-xs font-semibold tabular-nums text-white backdrop-blur">
          {{ imagePreview.index + 1 }} / {{ imagePreview.total }}
        </span>
        <span class="rounded-full bg-black/45 px-2 py-1 text-xs font-semibold text-white backdrop-blur">
          {{ Math.round(imagePreviewZoom * 100) }}%
        </span>
        <a
          class="flex h-11 w-11 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur transition hover:bg-white/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/80 sm:hidden"
          :href="imagePreview.url"
          :download="imagePreview.downloadName"
          target="_blank"
          rel="noopener"
          :title="t('playground.downloadImage')"
          :aria-label="t('playground.downloadImage')"
          @click.stop
        >
          <Icon name="download" size="md" />
        </a>
        <button
          type="button"
          class="rounded-full bg-black/45 p-2 text-white transition hover:bg-white/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/80"
          :title="t('common.close')"
          @click="closeImagePreview"
        >
          <Icon name="x" size="md" />
        </button>
      </div>
      <button
        v-if="imagePreview.total && imagePreview.total > 1"
        type="button"
        class="absolute left-3 top-1/2 z-20 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur transition hover:bg-white/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/80 sm:left-6"
        :title="t('playground.previousImage')"
        :aria-label="t('playground.previousImage')"
        @pointerdown.stop
        @click.stop="navigateImagePreview(-1)"
      >
        <Icon name="chevronLeft" size="lg" />
      </button>
      <button
        v-if="imagePreview.total && imagePreview.total > 1"
        type="button"
        class="absolute right-3 top-1/2 z-20 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full bg-black/45 text-white backdrop-blur transition hover:bg-white/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/80 sm:right-6"
        :title="t('playground.nextImage')"
        :aria-label="t('playground.nextImage')"
        @pointerdown.stop
        @click.stop="navigateImagePreview(1)"
      >
        <Icon name="chevronRight" size="lg" />
      </button>
      <img
        :src="imagePreview.url"
        :alt="imagePreview.title"
        class="pointer-events-auto absolute left-1/2 top-1/2 block max-w-none object-contain will-change-transform"
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { keysAPI } from '@/api/keys'
import {
  cancelPlaygroundRun,
  fetchModels,
  getPlaygroundRun,
  getPlaygroundRunImage,
  getPlaygroundRunVideo,
  startPlaygroundRun,
  streamChatCompletion,
  type PlaygroundChatMessage,
  type PlaygroundImageInput,
  type PlaygroundImageResult,
  type PlaygroundVideoResult,
  type PlaygroundModel,
  type PlaygroundRun,
  type PlaygroundRunRequest
} from '@/api/playground'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { formatDateOnly, formatRelativeTime, formatTime } from '@/utils/format'
import { platformIconClass, platformLabel } from '@/utils/platformColors'
import {
  buildImageBoardTasksFromThreads,
  buildVideoBoardTasksFromThreads,
  imageBoardRectsIntersect,
  type ImageBoardThreadTask,
  type VideoBoardThreadTask
} from '@/utils/playgroundBoardTools'
import { firstActualImageSize, firstImageDescription, gptImage2SizeFor, isGptImage2Model, mapClientPointToCanvas, wrapGalleryIndex } from '@/utils/playgroundImageTools'
import { toCloneablePlaygroundState } from '@/utils/playgroundPersistence'
import {
  isRecoverablePlaygroundError,
  isRetryablePlaygroundRequestError,
  markRecoverablePlaygroundError,
  playgroundRetryDelayMs
} from '@/utils/playgroundRunTools'
import type { ApiKey, GroupPlatform } from '@/types'

type PlaygroundMode = 'chat' | 'image' | 'video' | 'audio'
type MessageRole = 'user' | 'assistant'
type AttachmentKind = 'image' | 'text' | 'file'
type IconName = InstanceType<typeof Icon>['$props']['name']
type ImageSizeMode = 'auto' | 'ratio' | 'custom'
type ImageResolution = '1K' | '2K' | '4K'
type VideoResolution = '480p' | '720p' | '1080p'
type ImagePromptStyle = 'auto' | 'photo' | 'illustration' | 'anime' | 'cinematic' | 'product' | 'poster' | 'watercolor' | 'pixel'
type DoodleDrawingTool = 'brush' | 'eraser'
type DoodleTool = DoodleDrawingTool | 'sticker' | 'pan'
type ImageWorkspaceMode = 'chat' | 'board'
type ComposerPanel = 'model' | 'image' | 'video'
type PlaygroundRestorableRunRequest = Omit<PlaygroundRunRequest, 'apiKey'>

interface DoodlePoint {
  x: number
  y: number
}

interface DoodleStroke {
  kind: 'stroke'
  tool: DoodleDrawingTool
  color: string
  size: number
  opacity: number
  points: DoodlePoint[]
}

interface DoodleSticker {
  kind: 'sticker'
  id: string
  emoji: string
  size: number
  point: DoodlePoint
}

type DoodleOperation = DoodleStroke | DoodleSticker

interface DoodleEditorState {
  attachmentId: string
  name: string
  naturalWidth: number
  naturalHeight: number
}

interface PlaygroundAttachment {
  id: string
  name: string
  type: string
  size: number
  kind: AttachmentKind
  dataUrl?: string
  chatDataUrl?: string
  storageId?: string
  thumbnailUrl?: string
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
  videos?: PlaygroundVideoResult[]
  imageConfig?: PlaygroundImageConfig
  raw?: unknown
  pending?: boolean
  progress?: string
  runId?: string
  runStarted?: boolean
  runKeyId?: string
  runRequest?: PlaygroundRestorableRunRequest
  durationMs?: number
  error?: boolean
  favorite?: boolean
  videoDownloadProgress?: number | null
  videoRestoreUnavailable?: boolean
}

interface PlaygroundPersistedPayload {
  version: number
  savedAt: number
  activeThreadId: string
  mode: PlaygroundMode
  selectedKeyId: string
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
  imageWorkspaceMode?: ImageWorkspaceMode
  videoDuration?: number
  videoResolution?: VideoResolution
  videoAspectRatio?: string
  promptOptimizerKeyId?: string
  promptOptimizerModel: string
  showComposerConfig: boolean
  composerInputHeight: number
  composerInputHeightCustomized?: boolean
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

type PlaygroundImageBoardTask = ImageBoardThreadTask<PlaygroundMessage, PlaygroundThread>
type PlaygroundVideoBoardTask = VideoBoardThreadTask<PlaygroundMessage, PlaygroundThread>

interface ImageBoardSelection {
  startX: number
  startY: number
  currentX: number
  currentY: number
}

interface ImageBoardSelectionState {
  pointerId: number
  taskId: string | null
  additive: boolean
  initialSelectedIds: Set<string>
  startX: number
  startY: number
  didDrag: boolean
}

interface PlaygroundRunContext {
  mode: PlaygroundMode
  keyId: string
  apiKey: string
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
  videoDuration: number
  videoResolution: VideoResolution
  videoAspectRatio: string
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

interface PlaygroundFileSystemWritable {
  write(data: Blob): Promise<void>
  close(): Promise<void>
}

interface PlaygroundFileSystemFileHandle {
  getFile(): Promise<File>
  createWritable(): Promise<PlaygroundFileSystemWritable>
}

interface PlaygroundFileSystemDirectoryHandle {
  readonly name: string
  getFileHandle(name: string, options?: { create?: boolean }): Promise<PlaygroundFileSystemFileHandle>
  queryPermission?(options?: { mode: 'readwrite' }): Promise<PermissionState>
  requestPermission?(options?: { mode: 'readwrite' }): Promise<PermissionState>
}

type PlaygroundVideoDirectoryPermission = PermissionState | 'unsupported'

interface PlaygroundImagePersistBatch {
  records: PlaygroundPersistedImage[]
  applyThumbnails: () => void
}

interface PlaygroundAttachmentPersistBatch {
  records: PlaygroundPersistedImage[]
  persistedIds: string[]
  applyThumbnails: () => void
  markPersisted: () => void
}

interface PlaygroundRunHandle {
  controller: AbortController
  mode: PlaygroundMode
  threadId: string
  runId?: string
}

interface PlaygroundConversationExportImage {
  id: string
  dataUrl?: string
  url?: string
  mimeType?: string
  revisedPrompt?: string
  width?: number
  height?: number
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
  downloadName: string
  mimeType: string
  revisedPrompt?: string
  messageId: string
  index: number
  total: number
  ownsObjectUrl?: boolean
}

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

marked.setOptions({
  breaks: true,
  gfm: true
})

const apiKeys = ref<ApiKey[]>([])
const selectedKeyId = ref('')
const loadingKeys = ref(true)
const loadingModels = ref(false)
const showSettings = ref(false)
const showAssistantDetails = ref(true)
const showComposerConfig = ref(false)
const activeComposerPanel = ref<ComposerPanel | null>(null)
const composerModelSearch = ref('')
const composerInputHeight = ref(64)
const composerManualInputHeight = ref(64)
const composerInputHeightCustomized = ref(false)
const composerInputResizing = ref(false)
const composerDragActive = ref(false)
const showImageSizeModal = ref(false)
const showPromptOptimizerModal = ref(false)
const mode = ref<PlaygroundMode>('chat')
const imageWorkspaceMode = ref<ImageWorkspaceMode>('chat')
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
const videoDuration = ref(8)
const videoResolution = ref<VideoResolution>('720p')
const videoAspectRatio = ref('16:9')
const promptOptimizerKeyId = ref('')
const promptOptimizerModel = ref('')
const promptOptimizerModels = ref<PlaygroundModel[]>([])
const promptOptimizerDraftKeyId = ref('')
const promptOptimizerDraftModel = ref('')
const promptOptimizerDraftStyle = ref<ImagePromptStyle>('auto')
const loadingPromptOptimizerModels = ref(false)
const promptOptimizerModelLoadError = ref('')
const optimizingPrompt = ref(false)
let promptOptimizeAbortController: AbortController | null = null
const threads = ref<PlaygroundThread[]>([])
const activeThreadId = ref('')
const historySearch = ref('')
const mobileHistoryOpen = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const composerTextarea = ref<HTMLTextAreaElement | null>(null)
const historyImportInput = ref<HTMLInputElement | null>(null)
const messageScroller = ref<HTMLElement | null>(null)
const composerDock = ref<HTMLElement | null>(null)
const composerShell = ref<HTMLElement | null>(null)
const composerSpacerHeight = ref(260)
const imagePreviewViewport = ref<HTMLElement | null>(null)
const imagePreview = ref<PlaygroundImagePreview | null>(null)
const imageBoardRoot = ref<HTMLElement | null>(null)
const imageBoardDetailTask = ref<PlaygroundImageBoardTask | null>(null)
const imageBoardDetailImageIndex = ref(0)
const videoBoardDetailTask = ref<PlaygroundVideoBoardTask | null>(null)
const videoDirectoryHandle = shallowRef<PlaygroundFileSystemDirectoryHandle | null>(null)
const videoDirectoryPermission = ref<PlaygroundVideoDirectoryPermission>('prompt')
const imageBoardPage = ref(1)
const videoBoardPage = ref(1)
const selectedImageBoardTaskIds = ref<Set<string>>(new Set())
const showImageBoardDeleteConfirm = ref(false)
const imageBoardBatchDownloading = ref(false)
const imageBoardSelection = ref<ImageBoardSelection | null>(null)
const expandedImageDescriptions = ref<Set<string>>(new Set())
const imagePreviewZoom = ref(1)
const imagePreviewPanX = ref(0)
const imagePreviewPanY = ref(0)
const imagePreviewIsDragging = ref(false)
const imagePreviewNaturalWidth = ref(0)
const imagePreviewNaturalHeight = ref(0)
const imagePreviewViewportWidth = ref(0)
const imagePreviewViewportHeight = ref(0)
const suppressNextImagePreviewBackdropClick = ref(false)
const doodleEditor = ref<DoodleEditorState | null>(null)
const doodleEditorFullscreen = ref(false)
const doodleCanvas = ref<HTMLCanvasElement | null>(null)
const doodleCanvasViewport = ref<HTMLElement | null>(null)
const doodleTool = ref<DoodleTool>('brush')
const doodleCanvasPanning = ref(false)
const doodleCtrlPanning = ref(false)
const doodleStickerResizing = ref(false)
const doodleColor = ref('#ef4444')
const doodleBrushSize = ref(12)
const doodleBrushOpacity = ref(0.5)
const doodleZoom = ref(1)
const doodlePanX = ref(0)
const doodlePanY = ref(0)
const doodleCanvasWidth = ref(0)
const doodleCanvasHeight = ref(0)
const doodleCanvasViewportWidth = ref(0)
const doodleCanvasViewportHeight = ref(0)
const doodleStickerPanelOpen = ref(false)
const doodleSelectedSticker = ref('😀')
const selectedDoodleStickerId = ref<string | null>(null)
const doodleStrokes = ref<DoodleOperation[]>([])
const doodleRedoStrokes = ref<DoodleOperation[]>([])
const doodleSaving = ref(false)
let modelAbortController: AbortController | null = null
let promptOptimizerModelAbortController: AbortController | null = null
let persistedVideoHydrationController: AbortController | null = null
const runAbortControllers = new Map<string, PlaygroundRunHandle>()
const runRecoveryTimers = new Map<string, number>()
const runRecoveryAttempts = new Map<string, number>()
let persistTimer: number | undefined
let restoringState = false
let persistenceReady = false
let playgroundViewMounted = false
let composerResizeObserver: ResizeObserver | null = null
let imagePreviewResizeObserver: ResizeObserver | null = null
let doodleResizeObserver: ResizeObserver | null = null
let composerDragDepth = 0
let imagePreviewDragState: { pointerId: number; startX: number; startY: number; panX: number; panY: number } | null = null
let imagePreviewLoadToken = 0
let doodleSourceImage: HTMLImageElement | null = null
let doodleOverlayCanvas: HTMLCanvasElement | null = null
let doodlePointerId: number | null = null
let doodleStickerDragState: { pointerId: number; stickerId: string; offsetX: number; offsetY: number } | null = null
let doodleStickerResizeState: { pointerId: number; stickerId: string; startSize: number; startDistance: number } | null = null
let doodleCanvasPanState: { pointerId: number; startX: number; startY: number; panX: number; panY: number } | null = null
let doodleRenderFrame: number | null = null
let composerInputResizeState: { pointerId: number; startY: number; height: number } | null = null
let imageBoardSelectionState: ImageBoardSelectionState | null = null
let suppressImageBoardCardClickUntil = 0
let playgroundDBPromise: Promise<IDBDatabase> | null = null
const imageObjectURLs = new Set<string>()
const persistedAttachmentStorageIds = new Set<string>()
const attachmentThumbnailBlobCache = new Map<string, Blob>()

const PLAYGROUND_STORAGE_VERSION = 1
const PLAYGROUND_DB_NAME = 'sub2api-playground'
const PLAYGROUND_DB_VERSION = 3
const PLAYGROUND_STATE_STORE = 'states'
const PLAYGROUND_IMAGE_STORE = 'images'
const PLAYGROUND_HANDLE_STORE = 'handles'
const PLAYGROUND_IMAGE_URL_PREFIX = 'playground-image://'
const PLAYGROUND_STATE_UPDATED_EVENT = 'sub2api:playground-state-updated'
const PLAYGROUND_PENDING_IMAGE_TTL_MS = 60 * 60 * 1000
const PLAYGROUND_RUN_POLL_INTERVAL_MS = 900
const PLAYGROUND_RUN_POLL_MAX_MS = 45 * 60 * 1000
const PLAYGROUND_RUN_START_MAX_ATTEMPTS = 6
const PLAYGROUND_RUN_NOT_FOUND_MAX_ATTEMPTS = 10
const PLAYGROUND_IMAGE_FETCH_MAX_ATTEMPTS = 6
const PLAYGROUND_VIDEO_RESTORE_CONCURRENCY = 3
const PLAYGROUND_RUN_RECOVERY_BASE_MS = 5000
const PLAYGROUND_RUN_RECOVERY_MAX_MS = 60000
const PLAYGROUND_RUN_RECOVERY_MAX_ATTEMPTS = 3
const PLAYGROUND_CHAT_IMAGE_MAX_BYTES = 3.5 * 1024 * 1024
const PLAYGROUND_CHAT_IMAGE_EDGE_STEPS = [1568, 1280, 1024, 768]
const PLAYGROUND_CHAT_IMAGE_QUALITY_STEPS = [0.82, 0.72, 0.62, 0.52]
const PLAYGROUND_DOODLE_MAX_DISPLAY_EDGE = 1600
const PLAYGROUND_DOODLE_STICKER_MIN_SIZE = 24
const PLAYGROUND_DOODLE_STICKER_MAX_SIZE = 480
const PLAYGROUND_COMPOSER_MIN_HEIGHT = 64
const doodleColors = ['#ef4444', '#f97316', '#facc15', '#22c55e', '#0ea5e9', '#8b5cf6', '#ffffff', '#111827']
const doodleStickers = ['😀', '😍', '🤩', '😎', '🥳', '🔥', '✨', '⭐', '💥', '💯', '❤️', '👍', '👋', '🎨', '🎯', '🚀', '🌈', '🍀', '🌸', '💡', '🎉', '🎈', '🪄', '👑']
const playgroundInstanceId = `playground-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
const imageResolutionEdges: Record<ImageResolution, number> = {
  '1K': 1024,
  '2K': 2048,
  '4K': 4096
}
const GPT_IMAGE_2_MAX_DIMENSION = 3840
const supportsVideoDirectoryStorage = typeof window !== 'undefined' && typeof (window as Window & {
  showDirectoryPicker?: (options?: { mode?: 'readwrite' }) => Promise<PlaygroundFileSystemDirectoryHandle>
}).showDirectoryPicker === 'function'

const modeOptions = computed(() => [
  { value: 'chat' as const, label: t('playground.chatMode'), icon: 'chat' as IconName },
  { value: 'image' as const, label: t('playground.imageMode'), icon: 'sparkles' as IconName },
  { value: 'video' as const, label: t('playground.videoMode'), icon: 'play' as IconName },
  { value: 'audio' as const, label: t('playground.audioMode'), icon: 'cloud' as IconName }
])

const imageSizeOptions = computed(() => {
  const ratios = isGptImage2Model(selectedModel.value)
    ? ['1:1', '16:9', '9:16', '2:1', '1:2', '21:9', '9:21']
    : ['1:1', '3:2', '2:3', '16:9', '9:16', '4:3', '3:4', '21:9']
  return ratios.map((value) => ({ value, label: value }))
})

const imageSizeModeOptions = computed<Array<{ value: ImageSizeMode; label: string }>>(() => [
  { value: 'auto', label: t('playground.sizeAuto') },
  { value: 'ratio', label: t('playground.sizeByRatio') },
  { value: 'custom', label: t('playground.customWidthHeight') }
])

const imageResolutionOptions: ImageResolution[] = ['1K', '2K', '4K']
const videoDurationValues = [4, 5, 6, 8, 10] as const
const videoResolutionValues: VideoResolution[] = ['480p', '720p', '1080p']
const videoAspectRatioValues = ['16:9', '9:16', '1:1'] as const

const activeKeys = computed(() => apiKeys.value.filter((key) => String(key.status).toLowerCase() === 'active'))
const keySelectOptions = computed<SelectOption[]>(() => activeKeys.value.map((key) => ({
  value: String(key.id),
  label: key.name,
  platform: key.platform || key.group?.platform
})))
const promptOptimizerGroupOptions = computed<SelectOption[]>(() => activeKeys.value.map((key) => ({
  value: String(key.id),
  label: key.group?.name || key.name,
  description: key.group?.name && key.group.name !== key.name ? key.name : undefined,
  platform: key.platform || key.group?.platform
})))
const selectedKey = computed(() => activeKeys.value.find((key) => String(key.id) === selectedKeyId.value) || null)
const selectedPromptOptimizerKey = computed(() => activeKeys.value.find((key) => String(key.id) === promptOptimizerKeyId.value) || null)
const promptOptimizerDraftPlatform = computed<GroupPlatform | ''>(() => {
  const key = activeKeys.value.find((item) => String(item.id) === promptOptimizerDraftKeyId.value)
  return key?.platform || key?.group?.platform || ''
})
const activeThread = computed(() => threads.value.find((thread) => thread.id === activeThreadId.value) || null)
const currentMessages = computed(() => activeThread.value?.messages || [])
const imageBoardTasks = computed<PlaygroundImageBoardTask[]>(() => buildImageBoardTasksFromThreads(threads.value))
const videoBoardTasks = computed<PlaygroundVideoBoardTask[]>(() => buildVideoBoardTasksFromThreads(threads.value))
const imageBoardPageSize = 18
const imageBoardPageCount = computed(() => Math.max(1, Math.ceil(imageBoardTasks.value.length / imageBoardPageSize)))
const paginatedImageBoardTasks = computed(() => {
  const start = (imageBoardPage.value - 1) * imageBoardPageSize
  return imageBoardTasks.value.slice(start, start + imageBoardPageSize)
})
const videoBoardPageCount = computed(() => Math.max(1, Math.ceil(videoBoardTasks.value.length / imageBoardPageSize)))
const paginatedVideoBoardTasks = computed(() => {
  const start = (videoBoardPage.value - 1) * imageBoardPageSize
  return videoBoardTasks.value.slice(start, start + imageBoardPageSize)
})
const imageBoardPaginationItems = computed<Array<number | 'ellipsis'>>(() => {
  const total = imageBoardPageCount.value
  const current = imageBoardPage.value
  if (total <= 7) return Array.from({ length: total }, (_, index) => index + 1)
  const pages = new Set([1, total, current - 1, current, current + 1])
  const sortedPages = [...pages].filter((page) => page >= 1 && page <= total).sort((left, right) => left - right)
  const items: Array<number | 'ellipsis'> = []
  for (const page of sortedPages) {
    const previous = items[items.length - 1]
    if (typeof previous === 'number' && page - previous > 1) items.push('ellipsis')
    items.push(page)
  }
  return items
})
const videoBoardPaginationItems = computed<Array<number | 'ellipsis'>>(() => {
  const total = videoBoardPageCount.value
  const current = videoBoardPage.value
  if (total <= 7) return Array.from({ length: total }, (_, index) => index + 1)
  const pages = new Set([1, total, current - 1, current, current + 1])
  const sortedPages = [...pages].filter((page) => page >= 1 && page <= total).sort((left, right) => left - right)
  const items: Array<number | 'ellipsis'> = []
  for (const page of sortedPages) {
    const previous = items[items.length - 1]
    if (typeof previous === 'number' && page - previous > 1) items.push('ellipsis')
    items.push(page)
  }
  return items
})
watch(imageBoardPageCount, (pageCount) => {
  if (imageBoardPage.value > pageCount) {
    imageBoardPage.value = pageCount
    clearImageBoardSelection()
  }
})
watch(videoBoardPageCount, (pageCount) => {
  if (videoBoardPage.value > pageCount) videoBoardPage.value = pageCount
})
const isImageBoardMode = computed(() => mode.value === 'image' && imageWorkspaceMode.value === 'board')
const isVideoBoardMode = computed(() => mode.value === 'video')
const isBoardMode = computed(() => isImageBoardMode.value || isVideoBoardMode.value)
const imageBoardDetailImages = computed(() => imageBoardDetailTask.value?.message.images || [])
const imageBoardDetailCurrentImage = computed(() => imageBoardDetailImages.value[imageBoardDetailImageIndex.value] || null)
const imageBoardDetailRatioLabel = computed(() => imageBoardDetailTask.value?.message.imageConfig?.ratio || '')
const videoBoardDetailVideo = computed(() => videoBoardDetailTask.value?.message.videos?.[0] || null)
const imageBoardDetailReferenceAttachments = computed(() => {
  const task = imageBoardDetailTask.value
  if (!task) return []
  const sourceMessage = findImageReuseSourceMessage(task.message)
  const sourceAttachments = (sourceMessage?.attachments || []).filter((attachment) => {
    return attachment.kind === 'image' && Boolean(attachmentPreviewUrl(attachment))
  })
  if (sourceAttachments.length) return sourceAttachments
  return (task.message.runRequest?.images || [])
    .map((image, index): PlaygroundAttachment => ({
      id: `board-reference-${task.message.id}-${image.storageId || index}`,
      name: image.name || `image-${index + 1}.png`,
      type: image.type || '',
      size: 0,
      kind: 'image',
      dataUrl: image.dataUrl,
      storageId: image.storageId || storedImageIdFromURL(image.dataUrl || '')
    }))
    .filter((attachment) => Boolean(attachmentPreviewUrl(attachment)))
})
const imageBoardSelectionBoxStyle = computed(() => {
  const selection = imageBoardSelection.value
  if (!selection) return {}
  return {
    left: `${Math.min(selection.startX, selection.currentX)}px`,
    top: `${Math.min(selection.startY, selection.currentY)}px`,
    width: `${Math.abs(selection.currentX - selection.startX)}px`,
    height: `${Math.abs(selection.currentY - selection.startY)}px`
  }
})
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
const currentImageMaxDimension = computed(() => isGptImage2Model(effectiveModel.value) ? GPT_IMAGE_2_MAX_DIMENSION : 4096)
const effectiveImageSize = computed(() => {
  if (imageSizeMode.value === 'auto') return 'auto'
  if (imageSizeMode.value === 'custom') {
    return `${clampImageDimension(customImageWidth.value, currentImageMaxDimension.value)}x${clampImageDimension(customImageHeight.value, currentImageMaxDimension.value)}`
  }
  return calculateImageSize(imageResolution.value, imageRatio.value, effectiveModel.value)
})
const imagePreviewBaseSize = computed(() => {
  const naturalWidth = imagePreviewNaturalWidth.value || 1024
  const naturalHeight = imagePreviewNaturalHeight.value || 1024
  const viewportWidth = Math.max(imagePreviewViewportWidth.value, 320)
  const viewportHeight = Math.max(imagePreviewViewportHeight.value, 240)
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
const doodleCanvasBaseSize = computed(() => {
  const canvasWidth = doodleCanvasWidth.value || 1
  const canvasHeight = doodleCanvasHeight.value || 1
  const viewportWidth = Math.max(doodleCanvasViewportWidth.value, 1)
  const viewportHeight = Math.max(doodleCanvasViewportHeight.value, 1)
  const fitScale = Math.min(viewportWidth / canvasWidth, viewportHeight / canvasHeight, 1)
  return {
    width: Math.max(1, Math.round(canvasWidth * fitScale)),
    height: Math.max(1, Math.round(canvasHeight * fitScale))
  }
})
const doodleCanvasDisplaySize = computed(() => ({
  width: Math.max(1, Math.round(doodleCanvasBaseSize.value.width * doodleZoom.value)),
  height: Math.max(1, Math.round(doodleCanvasBaseSize.value.height * doodleZoom.value))
}))
const doodleCanvasCanPan = computed(() => {
  return doodleCanvasDisplaySize.value.width > doodleCanvasViewportWidth.value ||
    doodleCanvasDisplaySize.value.height > doodleCanvasViewportHeight.value
})
const doodleCanvasCursorClass = computed(() => {
  if (doodleStickerResizing.value) return 'cursor-se-resize'
  if (doodleCanvasPanning.value) return 'cursor-grabbing'
  return doodleCtrlPanning.value || doodleTool.value === 'pan' || doodleCanvasCanPan.value
    ? 'cursor-grab'
    : 'cursor-crosshair'
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

const playgroundNotice = computed(() => {
  if (modelLoadError.value) {
    return { type: 'warning' as const, message: modelLoadError.value }
  }
  return null
})

const chatModels = computed(() => models.value.filter((model) => model.id && !isImageModel(model.id) && !isVideoModel(model.id) && !isAudioModel(model.id)))
const imageModels = computed(() => models.value.filter((model) => model.id && isImageModel(model.id)))
const videoModels = computed(() => models.value.filter((model) => model.id && isVideoModel(model.id)))
const audioModels = computed(() => models.value.filter((model) => model.id && isAudioModel(model.id)))
const visibleModels = computed(() => {
  if (mode.value === 'image') return imageModels.value
  if (mode.value === 'video') return videoModels.value
  if (mode.value === 'audio') return audioModels.value
  return chatModels.value
})
const filteredComposerModels = computed(() => {
  const search = composerModelSearch.value.trim().toLowerCase()
  return visibleModels.value.filter((model) => {
    if (!search) return true
    return `${model.id} ${model.label || ''} ${model.owned_by || ''}`.toLowerCase().includes(search)
  })
})

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

const videoDurationSelectOptions = computed<SelectOption[]>(() => videoDurationValues.map((seconds) => ({
  value: seconds,
  label: t('playground.videoSeconds', { count: seconds })
})))

const videoResolutionSelectOptions = computed<SelectOption[]>(() => videoResolutionValues.map((resolution) => ({
  value: resolution,
  label: resolution
})))

const videoAspectRatioSelectOptions = computed<SelectOption[]>(() => videoAspectRatioValues.map((ratio) => ({
  value: ratio,
  label: ratio
})))

const outputFormatSegmentOptions = [
  { value: 'png', label: 'PNG' },
  { value: 'webp', label: 'WEBP' },
  { value: 'jpeg', label: 'JPEG' }
] as const

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

const promptOptimizerChatModels = computed(() => promptOptimizerModels.value.filter((model) => model.id && !isImageModel(model.id)))
const promptOptimizerModelOptions = computed<SelectOption[]>(() => promptOptimizerChatModels.value.map((model) => ({
  value: model.id,
  label: model.label || model.id,
  description: model.owned_by,
  platform: platformForModel(model.id, model.owned_by, promptOptimizerDraftPlatform.value)
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

function normalizeImageMimeType(mimeType: string): string {
  const normalized = mimeType.trim().toLowerCase()
  return normalized === 'image/jpg' ? 'image/jpeg' : normalized
}

function detectImageMimeTypeFromBytes(bytes: Uint8Array): string {
  if (
    bytes.length >= 8 &&
    bytes[0] === 0x89 &&
    bytes[1] === 0x50 &&
    bytes[2] === 0x4e &&
    bytes[3] === 0x47 &&
    bytes[4] === 0x0d &&
    bytes[5] === 0x0a &&
    bytes[6] === 0x1a &&
    bytes[7] === 0x0a
  ) return 'image/png'
  if (bytes.length >= 3 && bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
    return 'image/jpeg'
  }
  const ascii = String.fromCharCode(...bytes.slice(0, Math.min(bytes.length, 12)))
  if (ascii.startsWith('GIF87a') || ascii.startsWith('GIF89a')) return 'image/gif'
  if (bytes.length >= 12 && ascii.slice(0, 4) === 'RIFF' && ascii.slice(8, 12) === 'WEBP') {
    return 'image/webp'
  }
  return ''
}

function detectBase64ImageMimeType(base64Data: string): string {
  const prefix = base64Data.replace(/\s/g, '').slice(0, 64)
  if (!prefix) return ''
  const padded = prefix.padEnd(Math.ceil(prefix.length / 4) * 4, '=')
  try {
    const binary = atob(padded)
    const bytes = new Uint8Array(binary.length)
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index)
    }
    return detectImageMimeTypeFromBytes(bytes)
  } catch {
    return ''
  }
}

function normalizeImageDataURLHeader(dataUrl: string): string {
  const match = dataUrl.match(/^data:([^;,]+)?;base64,/i)
  if (!match) return dataUrl
  const declaredMimeType = normalizeImageMimeType(match[1] || '')
  if (!declaredMimeType.startsWith('image/')) return dataUrl
  const dataStart = match[0].length
  const actualMimeType = detectBase64ImageMimeType(dataUrl.slice(dataStart))
  if (!actualMimeType || actualMimeType === declaredMimeType) return dataUrl
  return `data:${actualMimeType};base64,${dataUrl.slice(dataStart)}`
}

function isPreferredChatImageDataURL(dataUrl: string): boolean {
  return /^data:image\/(?:jpeg|png);base64,/i.test(dataUrl)
}

function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(normalizeImageDataURLHeader(String(reader.result || '')))
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
    thread.pendingAttachments?.forEach((attachment) => {
      ensureAttachmentStorageId(attachment)
    })
    for (const message of thread.messages) {
      message.attachments?.forEach((attachment) => {
        ensureAttachmentStorageId(attachment)
      })
      message.images?.forEach((image, index) => {
        if (!image.storageId) {
          image.storageId = uid(`image-${message.id}-${index}`)
        }
      })
    }
  }
}

function ensureAttachmentStorageId(attachment: PlaygroundAttachment) {
  if (attachment.kind === 'image' && !attachment.storageId) {
    attachment.storageId = uid(`upload-${attachment.id || 'image'}`)
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

function revokeVideoObjectURLs(videos?: PlaygroundVideoResult[]) {
  for (const video of videos || []) revokeTrackedObjectURL(video.url)
}

function revokeAttachmentObjectURLs(attachments?: PlaygroundAttachment[]) {
  for (const attachment of attachments || []) {
    revokeTrackedObjectURL(attachment.thumbnailUrl)
    if (attachment.storageId) attachmentThumbnailBlobCache.delete(attachment.storageId)
  }
}

function revokeAllImageObjectURLs() {
  for (const url of imageObjectURLs) {
    URL.revokeObjectURL(url)
  }
  imageObjectURLs.clear()
}

async function createImageThumbnailBlob(blob: Blob, maxEdge = 640): Promise<Blob> {
  const image = await loadImageElement(blob)
  return renderImageBlob(image, blob, {
    maxEdge,
    mimeType: 'image/webp',
    quality: 0.82
  })
}

function loadImageElement(blob: Blob): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const objectUrl = URL.createObjectURL(blob)
    const element = new Image()
    element.onload = () => {
      URL.revokeObjectURL(objectUrl)
      resolve(element)
    }
    element.onerror = () => {
      URL.revokeObjectURL(objectUrl)
      reject(new Error('Failed to load image data'))
    }
    element.src = objectUrl
  })
}

function renderImageBlob(
  image: HTMLImageElement,
  fallbackBlob: Blob,
  options: {
    maxEdge: number
    mimeType: string
    quality: number
    fillStyle?: string
  }
): Promise<Blob> {
  const ratio = image.naturalWidth > 0 && image.naturalHeight > 0
    ? Math.min(1, options.maxEdge / Math.max(image.naturalWidth, image.naturalHeight))
    : 1
  const width = Math.max(1, Math.round((image.naturalWidth || options.maxEdge) * ratio))
  const height = Math.max(1, Math.round((image.naturalHeight || options.maxEdge) * ratio))
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const context = canvas.getContext('2d')
  if (!context) return Promise.resolve(fallbackBlob)
  if (options.fillStyle) {
    context.fillStyle = options.fillStyle
    context.fillRect(0, 0, width, height)
  }
  context.drawImage(image, 0, 0, width, height)
  return new Promise((resolve) => {
    canvas.toBlob((resized) => resolve(resized || fallbackBlob), options.mimeType, options.quality)
  })
}

async function createChatImageBlob(blob: Blob): Promise<Blob> {
  const image = await loadImageElement(blob)
  let bestBlob: Blob | null = null
  for (const maxEdge of PLAYGROUND_CHAT_IMAGE_EDGE_STEPS) {
    for (const quality of PLAYGROUND_CHAT_IMAGE_QUALITY_STEPS) {
      const candidate = await renderImageBlob(image, blob, {
        maxEdge,
        mimeType: 'image/jpeg',
        quality,
        fillStyle: '#fff'
      })
      if (!bestBlob || candidate.size < bestBlob.size) {
        bestBlob = candidate
      }
      if (candidate.size <= PLAYGROUND_CHAT_IMAGE_MAX_BYTES) {
        return candidate
      }
    }
  }
  return bestBlob || blob
}

async function createChatImageDataURL(blob: Blob, fallbackDataUrl = ''): Promise<string> {
  const chatBlob = await createChatImageBlob(blob)
  if (chatBlob === blob && fallbackDataUrl.startsWith('data:')) {
    return normalizeImageDataURLHeader(fallbackDataUrl)
  }
  return blobToDataURL(chatBlob)
}

function clampImageDimension(value: number, maxDimension = 4096): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return 1024
  return Math.min(Math.max(Math.round(parsed), 256), maxDimension)
}

function clampComposerInputHeight(value: number): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return PLAYGROUND_COMPOSER_MIN_HEIGHT
  return Math.min(Math.max(Math.round(parsed), PLAYGROUND_COMPOSER_MIN_HEIGHT), 360)
}

function calculateImageSize(resolution: ImageResolution, ratio: string, model: string): string {
  if (isGptImage2Model(model)) {
    return gptImage2SizeFor(resolution, ratio) || gptImage2SizeFor('2K', ratio) || '2048x2048'
  }
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

function isImageResolutionSupported(resolution: ImageResolution): boolean {
  return !isGptImage2Model(effectiveModel.value) || gptImage2SizeFor(resolution, imageRatio.value) !== null
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
  return value === 'anthropic' || value === 'openai' || value === 'gemini' || value === 'antigravity' || value === 'grok'
}

function optionPlatform(option: SelectOption | Record<string, unknown> | null): GroupPlatform | undefined {
  if (!option || typeof option !== 'object') return undefined
  const platform = option.platform
  return isGroupPlatform(platform) ? platform : undefined
}

function platformForModel(modelID: string, owner?: string, fallbackPlatform: GroupPlatform | '' = selectedKeyPlatform.value): GroupPlatform | undefined {
  const source = `${modelID} ${owner || ''}`.toLowerCase()
  if (source.includes('claude') || source.includes('anthropic')) return 'anthropic'
  if (source.includes('grok') || source.includes('xai') || source.includes('x.ai')) return 'grok'
  if (source.includes('gemini') || source.includes('google') || /(^|[-_/])veo([\d-]|$)/i.test(source)) return 'gemini'
  if (source.includes('antigravity')) return 'antigravity'
  if (/(\bgpt\b|gpt-|^o\d|dall-e|openai|image|seedance|doubao)/i.test(source)) return 'openai'
  return isGroupPlatform(fallbackPlatform) ? fallbackPlatform : undefined
}

function messagePlatform(message: PlaygroundMessage): GroupPlatform | undefined {
  if (message.platform) return message.platform
  return platformForModel(message.model || '')
}

function isImageModel(model: string): boolean {
  return /(^|[-_])(image|dall-e|flux|sd|midjourney)/i.test(model) || /^gpt-image-/i.test(model)
}

function isVideoModel(model: string): boolean {
  return /(^|[-_])(video|sora|gen-|veo-|seedance|doubao)/i.test(model) || /grok-imagine-video/i.test(model)
}

function isAudioModel(model: string): boolean {
  return /(^|[-_])(audio|tts|whisper|speech)/i.test(model)
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
        if (!db.objectStoreNames.contains(PLAYGROUND_HANDLE_STORE)) {
          db.createObjectStore(PLAYGROUND_HANDLE_STORE)
        }
      }
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error || new Error('Failed to open playground database'))
    })
  }
  return playgroundDBPromise
}

function videoDirectoryHandleKey(): string {
  return `${storageKey()}:video-directory`
}

async function saveVideoDirectoryHandleToDB(handle: PlaygroundFileSystemDirectoryHandle) {
  const db = await openPlaygroundDB()
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction(PLAYGROUND_HANDLE_STORE, 'readwrite')
    transaction.objectStore(PLAYGROUND_HANDLE_STORE).put(handle, videoDirectoryHandleKey())
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error || new Error('Failed to persist video directory handle'))
  })
}

async function loadVideoDirectoryHandleFromDB(): Promise<PlaygroundFileSystemDirectoryHandle | null> {
  if (!supportsVideoDirectoryStorage) return null
  const db = await openPlaygroundDB()
  return await new Promise((resolve, reject) => {
    const transaction = db.transaction(PLAYGROUND_HANDLE_STORE, 'readonly')
    const request = transaction.objectStore(PLAYGROUND_HANDLE_STORE).get(videoDirectoryHandleKey())
    request.onsuccess = () => resolve((request.result as PlaygroundFileSystemDirectoryHandle | undefined) || null)
    request.onerror = () => reject(request.error || new Error('Failed to read video directory handle'))
  })
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

async function deletePlaygroundImageFromDB(id: string): Promise<void> {
  if (!id) return
  const db = await openPlaygroundDB()
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction(PLAYGROUND_IMAGE_STORE, 'readwrite')
    transaction.objectStore(PLAYGROUND_IMAGE_STORE).delete(id)
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error || new Error('Failed to delete playground image'))
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

async function collectPersistedAttachmentsFromThreads(): Promise<PlaygroundAttachmentPersistBatch> {
  const records: PlaygroundPersistedImage[] = []
  const persistedIds: string[] = []
  const replacements: Array<{ attachment: PlaygroundAttachment; thumbnailBlob: Blob }> = []
  for (const thread of threads.value) {
    for (const attachment of [
      ...(thread.pendingAttachments || []),
      ...thread.messages.flatMap((message) => message.attachments || [])
    ]) {
      if (attachment.kind !== 'image') continue
      ensureAttachmentStorageId(attachment)
      if (attachment.storageId && persistedAttachmentStorageIds.has(attachment.storageId)) continue
      if (!attachment.storageId || !attachment.dataUrl?.startsWith('data:')) continue
      const blob = dataURLToBlob(attachment.dataUrl)
      if (!blob) continue
      const cachedThumbnailBlob = attachmentThumbnailBlobCache.get(attachment.storageId)
      const thumbnailBlob = cachedThumbnailBlob || (!attachment.thumbnailUrl
        ? await createImageThumbnailBlob(blob).catch(() => blob)
        : undefined)
      attachment.type = attachment.type || blob.type
      if (thumbnailBlob && !attachment.thumbnailUrl) {
        attachmentThumbnailBlobCache.set(attachment.storageId, thumbnailBlob)
        replacements.push({ attachment, thumbnailBlob })
      }
      persistedIds.push(attachment.storageId)
      records.push({
        id: attachment.storageId,
        blob,
        thumbnailBlob,
        mimeType: attachment.type || blob.type,
        savedAt: Date.now()
      })
    }
  }
  return {
    records,
    persistedIds,
    applyThumbnails() {
      for (const replacement of replacements) {
        replacement.attachment.thumbnailUrl = createTrackedObjectURL(replacement.thumbnailBlob)
      }
    },
    markPersisted() {
      for (const id of persistedIds) {
        persistedAttachmentStorageIds.add(id)
        attachmentThumbnailBlobCache.delete(id)
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
    attachments: message.attachments?.map(serializeAttachmentForPersistence),
    images: message.images?.map((image) => ({
      ...image,
      url: image.storageId ? storedImagePlaceholder(image.storageId) : image.url,
      thumbnailUrl: undefined
    })) || [],
    videos: message.videos?.map((video, index) => ({
      ...video,
      url: '',
      thumbnailUrl: undefined,
      assetIndex: Number.isInteger(video.assetIndex) ? video.assetIndex : index
    })) || [],
    pending: recoverablePendingRun || recoverablePendingImage,
    raw: undefined,
    progress: recoverablePendingRun || recoverablePendingImage
      ? (message.progress || (message.imageConfig ? t('playground.generatingImages') : t('playground.streaming')))
      : '',
    content: normalizeRestoredMessageContent(message)
  }
}

function serializeAttachmentForPersistence(attachment: PlaygroundAttachment): PlaygroundAttachment {
  if (attachment.kind !== 'image') return attachment
  return {
    ...attachment,
    dataUrl: attachment.storageId ? storedImagePlaceholder(attachment.storageId) : attachment.dataUrl,
    thumbnailUrl: undefined
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

async function runVideoRestoreTasks(tasks: Array<() => Promise<void>>) {
  let nextTask = 0
  const workerCount = Math.min(PLAYGROUND_VIDEO_RESTORE_CONCURRENCY, tasks.length)
  await Promise.all(Array.from({ length: workerCount }, async () => {
    while (nextTask < tasks.length) {
      const taskIndex = nextTask
      nextTask += 1
      await tasks[taskIndex]()
    }
  }))
}

async function hydratePersistedVideos(signal?: AbortSignal) {
  const hydrationSignal = signal || new AbortController().signal
  const restoreTasks: Array<() => Promise<void>> = []
  for (const thread of threads.value) {
    for (const message of thread.messages) {
      if (!message.runId || !message.videos?.length) continue
      const knownUnavailable = Boolean(message.videoRestoreUnavailable)
      // Blob URLs belong to the document that created them and cannot be
      // trusted after a reload, even when the persisted string still starts
      // with "blob:".
      const videos = message.videos.map((video, index) => ({
        ...video,
        url: '',
        thumbnailUrl: undefined,
        assetIndex: Number.isInteger(video.assetIndex) ? video.assetIndex : index
      }))
      if (knownUnavailable && !videos.some((video) => video.localFileName)) {
        revokeVideoObjectURLs(message.videos)
        message.videos = videos
        message.videoDownloadProgress = undefined
        message.progress = ''
        message.content = t('playground.videoCacheMissing')
        message.error = true
        continue
      }
      message.videoDownloadProgress = null
      message.progress = t('playground.videoDownloading')
      message.content = t('playground.videoDownloading')
      message.error = false
      restoreTasks.push(async () => {
        if (hydrationSignal.aborted) return
        try {
          const restoredVideos = await hydratePlaygroundRunVideos({
            id: message.runId as string,
            mode: 'video',
            status: 'succeeded',
            videos
          }, hydrationSignal, (progress) => {
            message.videoDownloadProgress = progress
            message.progress = progress === null
              ? t('playground.videoDownloading')
              : t('playground.videoDownloadingProgress', { progress: Math.round(progress) })
          }, { retry: false, allowRemote: !knownUnavailable })
          message.videos = restoredVideos
          message.videoDownloadProgress = undefined
          message.progress = ''
          message.content = restoredVideos.length > 0
            ? t('playground.videoGenerated')
            : t('playground.videoCacheMissing')
          message.error = restoredVideos.length === 0
          message.videoRestoreUnavailable = false
        } catch (error) {
          if (hydrationSignal.aborted) return
          if (!knownUnavailable) console.warn('Failed to restore playground video:', error)
          revokeVideoObjectURLs(message.videos)
          // Keep localFileName/assetIndex so granting folder access or a later
          // provider URL retry can recover the video without regeneration.
          message.videos = videos
          message.videoDownloadProgress = undefined
          message.progress = ''
          message.content = t('playground.videoCacheMissing')
          message.error = true
          if (playgroundRequestStatus(error) === 404) message.videoRestoreUnavailable = true
        }
      })
    }
  }
  await runVideoRestoreTasks(restoreTasks)
}

async function queryVideoDirectoryPermission(
  handle: PlaygroundFileSystemDirectoryHandle,
  requestAccess = false,
): Promise<PermissionState> {
  const options = { mode: 'readwrite' as const }
  let permission = handle.queryPermission ? await handle.queryPermission(options) : 'granted'
  if (permission !== 'granted' && requestAccess && handle.requestPermission) {
    permission = await handle.requestPermission(options)
  }
  videoDirectoryPermission.value = permission
  return permission
}

async function restoreVideoDirectoryHandle() {
  if (!supportsVideoDirectoryStorage) {
    videoDirectoryPermission.value = 'unsupported'
    return
  }
  try {
    const handle = await loadVideoDirectoryHandleFromDB()
    videoDirectoryHandle.value = handle
    videoDirectoryPermission.value = handle
      ? await queryVideoDirectoryPermission(handle)
      : 'prompt'
  } catch (error) {
    console.warn('Failed to restore video directory handle:', error)
    videoDirectoryHandle.value = null
    videoDirectoryPermission.value = 'prompt'
  }
}

function videoFileExtension(mimeType: string): string {
  const normalized = mimeType.toLowerCase()
  if (normalized.includes('webm')) return 'webm'
  if (normalized.includes('quicktime')) return 'mov'
  return 'mp4'
}

function videoLocalFileName(runId: string, assetIndex: number, mimeType: string): string {
  const safeRunId = runId.replace(/[^a-zA-Z0-9_-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 80) || 'video'
  return `sub2api-${safeRunId}-${assetIndex + 1}.${videoFileExtension(mimeType)}`
}

async function saveVideoBlobToDirectory(blob: Blob, fileName: string): Promise<boolean> {
  const handle = videoDirectoryHandle.value
  if (!handle || await queryVideoDirectoryPermission(handle) !== 'granted') return false
  const fileHandle = await handle.getFileHandle(fileName, { create: true })
  const writable = await fileHandle.createWritable()
  try {
    await writable.write(blob)
  } finally {
    await writable.close()
  }
  return true
}

async function loadVideoBlobFromDirectory(fileName?: string): Promise<Blob | null> {
  const handle = videoDirectoryHandle.value
  if (!handle || !fileName || await queryVideoDirectoryPermission(handle) !== 'granted') return null
  try {
    const fileHandle = await handle.getFileHandle(fileName)
    return await fileHandle.getFile()
  } catch (error) {
    if ((error as DOMException)?.name !== 'NotFoundError') {
      console.warn('Failed to read locally persisted video:', error)
    }
    return null
  }
}

async function persistLoadedVideosToSelectedDirectory() {
  const jobs: Array<Promise<void>> = []
  for (const thread of threads.value) {
    for (const message of thread.messages) {
      if (!message.runId || !message.videos?.length) continue
      message.videos.forEach((video, index) => {
        if (!video.url) return
        jobs.push((async () => {
          const response = await fetch(video.url)
          if (!response.ok) throw new Error(`HTTP ${response.status}`)
          const blob = await response.blob()
          const mimeType = video.mimeType || blob.type || 'video/mp4'
          const fileName = video.localFileName || videoLocalFileName(message.runId as string, index, mimeType)
          if (await saveVideoBlobToDirectory(blob, fileName)) video.localFileName = fileName
        })())
      })
    }
  }
  const results = await Promise.allSettled(jobs)
  for (const result of results) {
    if (result.status === 'rejected') {
      console.warn('Failed to move an existing video into the selected directory:', result.reason)
    }
  }
}

async function restoreUnavailableVideosAfterDirectoryPermission() {
  const jobs: Array<Promise<void>> = []
  for (const thread of threads.value) {
    for (const message of thread.messages) {
      if (!message.runId || !message.videos?.length || message.videos.some((video) => video.url)) continue
      const storedVideos = message.videos.map((video, index) => ({
        ...video,
        assetIndex: Number.isInteger(video.assetIndex) ? video.assetIndex : index,
      }))
      jobs.push((async () => {
        const controller = new AbortController()
        try {
          const restored = await hydratePlaygroundRunVideos({
            id: message.runId as string,
            mode: 'video',
            status: 'succeeded',
            videos: storedVideos,
          }, controller.signal)
          if (restored.length === 0) return
          message.videos = restored
          message.error = false
          message.videoRestoreUnavailable = false
          message.progress = ''
          message.content = t('playground.videoGenerated')
        } catch (error) {
          console.warn('Failed to restore video after directory permission was granted:', error)
        }
      })())
    }
  }
  await Promise.all(jobs)
}

async function chooseVideoStorageDirectory() {
  if (!supportsVideoDirectoryStorage) {
    appStore.showError(t('playground.videoFolderUnsupported'))
    return
  }
  const picker = (window as Window & {
    showDirectoryPicker?: (options?: { mode?: 'readwrite' }) => Promise<PlaygroundFileSystemDirectoryHandle>
  }).showDirectoryPicker
  if (!picker) return
  try {
    const currentHandle = videoDirectoryHandle.value
    if (currentHandle && await queryVideoDirectoryPermission(currentHandle, true) === 'granted') {
      await restoreUnavailableVideosAfterDirectoryPermission()
      await persistLoadedVideosToSelectedDirectory()
      persistPlaygroundState()
      appStore.showSuccess(t('playground.videoFolderSelected', { name: currentHandle.name }))
      return
    }
    const handle = await picker({ mode: 'readwrite' })
    if (await queryVideoDirectoryPermission(handle, true) !== 'granted') return
    await saveVideoDirectoryHandleToDB(handle)
    videoDirectoryHandle.value = handle
    await restoreUnavailableVideosAfterDirectoryPermission()
    await persistLoadedVideosToSelectedDirectory()
    persistPlaygroundState()
    appStore.showSuccess(t('playground.videoFolderSelected', { name: handle.name }))
  } catch (error) {
    if ((error as DOMException)?.name === 'AbortError') return
    console.warn('Failed to select video storage directory:', error)
    appStore.showError(t('playground.videoFolderSelectFailed'))
  }
}

const videoDirectoryStatusLabel = computed(() => {
  if (!supportsVideoDirectoryStorage || videoDirectoryPermission.value === 'unsupported') {
    return t('playground.videoFolderUnsupported')
  }
  if (!videoDirectoryHandle.value) return t('playground.videoFolderNotSelected')
  if (videoDirectoryPermission.value !== 'granted') {
    return t('playground.videoFolderNeedsPermission', { name: videoDirectoryHandle.value.name })
  }
  return t('playground.videoFolderSelected', { name: videoDirectoryHandle.value.name })
})

const videoDirectoryActionLabel = computed(() => (
  videoDirectoryHandle.value && videoDirectoryPermission.value !== 'granted'
    ? t('playground.videoFolderReauthorize')
    : t('playground.videoFolderChoose')
))

async function hydratePersistedAttachments() {
  const restoreJobs: Array<Promise<void>> = []
  for (const thread of threads.value) {
    for (const attachment of [
      ...(thread.pendingAttachments || []),
      ...thread.messages.flatMap((message) => message.attachments || [])
    ]) {
      if (attachment.kind !== 'image') continue
      const storageId = attachment.storageId || storedImageIdFromURL(attachment.dataUrl || '')
      if (!storageId) continue
      attachment.storageId = storageId
      restoreJobs.push(loadPlaygroundImageFromDB(storageId)
        .then(async (persisted) => {
          if (!persisted) return
          if (persisted.thumbnailBlob) {
            attachment.thumbnailUrl = createTrackedObjectURL(persisted.thumbnailBlob)
          } else if (persisted.blob) {
            const thumbnailBlob = await createImageThumbnailBlob(persisted.blob).catch(() => persisted.blob as Blob)
            attachment.thumbnailUrl = createTrackedObjectURL(thumbnailBlob)
          }
          if (persisted.blob) {
            attachment.dataUrl = await blobToDataURL(persisted.blob)
          } else if (!attachment.dataUrl || attachment.dataUrl.startsWith(PLAYGROUND_IMAGE_URL_PREFIX)) {
            attachment.dataUrl = undefined
          }
          attachment.type = attachment.type || persisted.mimeType || persisted.blob?.type || ''
          persistedAttachmentStorageIds.add(storageId)
        })
        .catch((error) => {
          console.warn('Failed to restore playground attachment image:', error)
        }))
    }
  }
  await Promise.all(restoreJobs)
  for (const thread of threads.value) {
    for (const message of thread.messages) {
      if (!message.runRequest?.images?.length || !message.attachments?.length) continue
      const imagesByStorageId = new Map(
        message.attachments
          .filter((attachment) => attachment.kind === 'image' && attachment.storageId && attachment.dataUrl?.startsWith('data:'))
          .map((attachment) => [attachment.storageId as string, attachment])
      )
      message.runRequest.images = message.runRequest.images.map((image) => {
        const storageId = image.storageId || storedImageIdFromURL(image.dataUrl || '')
        const attachment = storageId ? imagesByStorageId.get(storageId) : undefined
        return attachment?.dataUrl
          ? { ...image, storageId, dataUrl: attachment.dataUrl }
          : image
      })
    }
  }
}

async function savePlaygroundStateToDB(payload: PlaygroundPersistedPayload) {
  const db = await openPlaygroundDB()
  const cloneablePayload = toCloneablePlaygroundState(payload)
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction(PLAYGROUND_STATE_STORE, 'readwrite')
    transaction.objectStore(PLAYGROUND_STATE_STORE).put(cloneablePayload, storageKey())
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
    imageWorkspaceMode: imageWorkspaceMode.value,
    videoDuration: videoDuration.value,
    videoResolution: videoResolution.value,
    videoAspectRatio: videoAspectRatio.value,
    promptOptimizerKeyId: promptOptimizerKeyId.value,
    promptOptimizerModel: promptOptimizerModel.value,
    showComposerConfig: showComposerConfig.value,
    composerInputHeight: composerManualInputHeight.value,
    composerInputHeightCustomized: composerInputHeightCustomized.value,
    threads: threads.value.map((thread) => ({
      ...thread,
      running: false,
      pendingAttachments: (thread.pendingAttachments || []).map(serializeAttachmentForPersistence),
      messages: thread.messages.map((message) => ({
        ...message,
        attachments: message.attachments?.map(serializeAttachmentForPersistence),
        runRequest: Boolean(message.pending && message.runId) || isRecoverablePendingImageMessage(message)
          ? message.runRequest
          : undefined,
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
      pendingAttachments: (thread.pendingAttachments || []).map(stripAttachmentDataForLocalStorage),
      messages: thread.messages.map((message) => ({
        ...message,
        attachments: message.attachments?.map(stripAttachmentDataForLocalStorage),
        runRequest: stripRunRequestImageDataForLocalStorage(message.runRequest),
        images: message.images?.map((image) => ({
          ...image,
          url: image.storageId ? storedImagePlaceholder(image.storageId) : image.url
        })).filter((image) => image.url || image.storageId) || []
      }))
    }))
  }
}

function stripAttachmentDataForLocalStorage(attachment: PlaygroundAttachment): PlaygroundAttachment {
  if (attachment.kind !== 'image') return attachment
  return {
    ...attachment,
    dataUrl: attachment.storageId ? storedImagePlaceholder(attachment.storageId) : undefined,
    thumbnailUrl: undefined
  }
}

function stripRunRequestImageDataForLocalStorage(request?: PlaygroundRestorableRunRequest): PlaygroundRestorableRunRequest | undefined {
  if (!request?.images?.length) return request
  return {
    ...request,
    images: request.images.map((image) => ({
      ...image,
      dataUrl: image.storageId ? storedImagePlaceholder(image.storageId) : ''
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
  imageWorkspaceMode.value = payload.imageWorkspaceMode === 'board' ? 'board' : 'chat'
  const savedVideoDuration = Number(payload.videoDuration)
  videoDuration.value = videoDurationValues.includes(savedVideoDuration as typeof videoDurationValues[number])
    ? savedVideoDuration
    : videoDuration.value
  videoResolution.value = videoResolutionValues.includes(payload.videoResolution as VideoResolution)
    ? payload.videoResolution as VideoResolution
    : videoResolution.value
  videoAspectRatio.value = videoAspectRatioValues.includes(payload.videoAspectRatio as typeof videoAspectRatioValues[number])
    ? payload.videoAspectRatio as string
    : videoAspectRatio.value
  promptOptimizerKeyId.value = typeof payload.promptOptimizerKeyId === 'string' ? payload.promptOptimizerKeyId : promptOptimizerKeyId.value
  promptOptimizerModel.value = typeof payload.promptOptimizerModel === 'string' ? payload.promptOptimizerModel : promptOptimizerModel.value
  showComposerConfig.value = typeof payload.showComposerConfig === 'boolean' ? payload.showComposerConfig : showComposerConfig.value
  composerInputHeightCustomized.value = payload.composerInputHeightCustomized === true
  composerManualInputHeight.value = composerInputHeightCustomized.value && typeof payload.composerInputHeight === 'number'
    ? clampComposerInputHeight(payload.composerInputHeight)
    : PLAYGROUND_COMPOSER_MIN_HEIGHT
  composerInputHeight.value = composerManualInputHeight.value
}

async function writePlaygroundStateNow(requireDurablePersistence = false) {
  if (restoringState) {
    if (requireDurablePersistence) throw new Error('Playground state is still restoring')
    return
  }
  if (persistTimer !== undefined) {
    window.clearTimeout(persistTimer)
    persistTimer = undefined
  }
  ensureImageStorageIds()
  const imageBatch = await collectPersistedImagesFromThreads()
  const attachmentBatch = await collectPersistedAttachmentsFromThreads()
  const payload = buildPlaygroundPayload()
  let assetsPersisted = false
  let databaseStatePersisted = false
  let persistenceError: unknown
  try {
    await savePlaygroundImagesToDB([...imageBatch.records, ...attachmentBatch.records])
    assetsPersisted = true
    imageBatch.applyThumbnails()
    attachmentBatch.applyThumbnails()
    attachmentBatch.markPersisted()
    await savePlaygroundStateToDB(payload)
    databaseStatePersisted = true
    localStorage.setItem(storageKey(), JSON.stringify(buildLocalStoragePayload(payload)))
    window.dispatchEvent(new CustomEvent(PLAYGROUND_STATE_UPDATED_EVENT, { detail: { key: storageKey(), source: playgroundInstanceId } }))
  } catch (error) {
    persistenceError = error
    console.warn('Failed to persist playground state:', error)
    try {
      localStorage.setItem(storageKey(), JSON.stringify(buildLocalStoragePayload(payload)))
    } catch (fallbackError) {
      console.warn('Failed to persist playground fallback state:', fallbackError)
    }
  }
  if (requireDurablePersistence && (!assetsPersisted || !databaseStatePersisted)) {
    throw persistenceError instanceof Error ? persistenceError : new Error('Failed to persist playground state')
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

async function restorePlaygroundState(hydrateVideos = true) {
  restoringState = true
  try {
    await restoreVideoDirectoryHandle()
    const dbPayload = await loadPlaygroundStateFromDB().catch((error) => {
      console.warn('Failed to load playground state from IndexedDB:', error)
      return null
    })
    if (dbPayload) {
      applyPlaygroundPayload(dbPayload as unknown as Record<string, unknown>)
      await hydratePersistedImages()
      if (hydrateVideos) await hydratePersistedVideos()
      await hydratePersistedAttachments()
      return
    }
    const raw = localStorage.getItem(storageKey())
    if (!raw) return
    applyPlaygroundPayload(JSON.parse(raw) as Record<string, unknown>)
    await hydratePersistedImages()
    if (hydrateVideos) await hydratePersistedVideos()
    await hydratePersistedAttachments()
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
    await hydratePersistedVideos()
    await hydratePersistedAttachments()
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

function imageBoardTaskDate(timestamp: number): string {
  const date = new Date(timestamp)
  return `${formatDateOnly(date)} ${formatMessageTime(timestamp)}`
}

function renderMessageMarkdown(content: string): string {
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
}

function imageMessageSizeLabel(message: PlaygroundMessage, image?: PlaygroundImageResult): string {
  const actualSize = firstActualImageSize(image ? [image] : message.images)
  if (actualSize) return actualSize
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

function promptOptimizerGroupLabel(keyId = promptOptimizerKeyId.value): string {
  const key = activeKeys.value.find((item) => String(item.id) === keyId)
  return key?.group?.name || key?.name || t('playground.selectOptimizerGroup')
}

function openPromptOptimizerSettings() {
  const savedKeyIsActive = activeKeys.value.some((key) => String(key.id) === promptOptimizerKeyId.value)
  promptOptimizerDraftKeyId.value = savedKeyIsActive
    ? promptOptimizerKeyId.value
    : (activeKeys.value[0] ? String(activeKeys.value[0].id) : '')
  promptOptimizerDraftModel.value = promptOptimizerModel.value
  promptOptimizerDraftStyle.value = imagePromptStyle.value
  showPromptOptimizerModal.value = true
  void loadPromptOptimizerModels(promptOptimizerDraftKeyId.value)
}

function closePromptOptimizerSettings() {
  promptOptimizerModelAbortController?.abort()
  showPromptOptimizerModal.value = false
}

function savePromptOptimizerSettings() {
  const optimizerKeyExists = activeKeys.value.some((key) => String(key.id) === promptOptimizerDraftKeyId.value)
  if (!optimizerKeyExists) {
    appStore.showInfo(t('playground.selectOptimizerGroupFirst'))
    return
  }
  if (!promptOptimizerDraftModel.value.trim()) {
    appStore.showInfo(t('playground.selectOptimizerModelFirst'))
    return
  }
  promptOptimizerKeyId.value = promptOptimizerDraftKeyId.value
  promptOptimizerModel.value = promptOptimizerDraftModel.value.trim()
  imagePromptStyle.value = promptOptimizerDraftStyle.value
  closePromptOptimizerSettings()
  appStore.showSuccess(t('common.saved'))
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
  mobileHistoryOpen.value = false
  selectDefaultModel()
  scrollMessagesToBottom()
}

function closeImageBoardDetail() {
  imageBoardDetailTask.value = null
  imageBoardDetailImageIndex.value = 0
}

function closeVideoBoardDetail() {
  videoBoardDetailTask.value = null
}

function openImageBoardDetailImage(index: number) {
  const task = imageBoardDetailTask.value
  if (!task) return
  closeImageBoardDetail()
  void openImagePreview(task.message, index)
}

function navigateImageBoardDetail(direction: -1 | 1) {
  const total = imageBoardDetailImages.value.length
  if (total <= 1) return
  imageBoardDetailImageIndex.value = (imageBoardDetailImageIndex.value + direction + total) % total
}

async function downloadImageBoardDetailImage() {
  const task = imageBoardDetailTask.value
  const image = imageBoardDetailCurrentImage.value
  if (!task || !image) return
  try {
    const blob = await resolveGeneratedImageBlob(image)
    if (!blob) throw new Error(t('playground.imageCacheMissing'))
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = imagePreviewDownloadName(task.message.createdAt, imageBoardDetailImageIndex.value, blob.type || image.mimeType || 'image/png')
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  } catch (error) {
    appStore.showError((error as Error)?.message || t('playground.imageCacheMissing'))
  }
}

function activateImageBoardTaskThread(task: PlaygroundImageBoardTask) {
  if (activeThreadId.value !== task.thread.id) {
    selectThread(task.thread.id)
  }
}

async function redrawImageBoardTask(task: PlaygroundImageBoardTask, index: number) {
  const image = task.message.images?.[index]
  if (!image) return
  closeImageBoardDetail()
  activateImageBoardTaskThread(task)
  await redrawGeneratedImage(task.message, image, index)
}

async function reuseImageBoardTask(task: PlaygroundImageBoardTask) {
  await reuseImageConfig(task.message)
  closeImageBoardDetail()
}

function isImageBoardTaskSelected(taskId: string): boolean {
  return selectedImageBoardTaskIds.value.has(taskId)
}

function imageBoardTaskSource(task: PlaygroundImageBoardTask): string {
  const platform = messagePlatform(task.message)
  return platform ? platformLabel(platform) : task.thread.title
}

function toggleImageBoardTaskFavorite(task: PlaygroundImageBoardTask) {
  task.message.favorite = !task.message.favorite
}

function toggleImageBoardTaskSelection(taskId: string) {
  const task = imageBoardTasks.value.find((item) => item.message.id === taskId)
  if (!task || task.message.pending) return
  const next = new Set(selectedImageBoardTaskIds.value)
  if (next.has(taskId)) next.delete(taskId)
  else next.add(taskId)
  selectedImageBoardTaskIds.value = next
}

function clearImageBoardSelection() {
  selectedImageBoardTaskIds.value = new Set()
}

function selectImageBoardPage(nextPage: number) {
  const page = Math.min(Math.max(1, nextPage), imageBoardPageCount.value)
  if (page === imageBoardPage.value) return
  imageBoardPage.value = page
  clearImageBoardSelection()
  void nextTick(() => messageScroller.value?.scrollTo({ top: 0, behavior: 'smooth' }))
}

function selectVideoBoardPage(nextPage: number) {
  const page = Math.min(Math.max(1, nextPage), videoBoardPageCount.value)
  if (page === videoBoardPage.value) return
  videoBoardPage.value = page
  void nextTick(() => messageScroller.value?.scrollTo({ top: 0, behavior: 'smooth' }))
}

function deleteImageBoardTask(task: PlaygroundImageBoardTask) {
  deleteMessageFromThread(task.thread, task.message.id)
}

function requestDeleteSelectedImageBoardTasks() {
  if (selectedImageBoardTaskIds.value.size === 0) return
  showImageBoardDeleteConfirm.value = true
}

async function confirmDeleteSelectedImageBoardTasks() {
  const selectedIds = new Set(selectedImageBoardTaskIds.value)
  showImageBoardDeleteConfirm.value = false
  for (const task of imageBoardTasks.value) {
    if (selectedIds.has(task.message.id)) deleteMessageFromThread(task.thread, task.message.id)
  }
  clearImageBoardSelection()
  await writePlaygroundStateNow()
}

function triggerPlaygroundBlobDownload(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

function imageBoardBatchFileName(
  task: PlaygroundImageBoardTask,
  imageIndex: number,
  mimeType: string,
  sequence: number
): string {
  const date = new Date(Number.isFinite(task.message.createdAt) ? task.message.createdAt : Date.now())
  const stamp = date.toISOString().replace(/\D/g, '').slice(0, 14)
  const taskID = task.message.id.replace(/[^a-zA-Z0-9_-]/g, '').slice(-12) || 'image'
  const extension = imageFileExtension(normalizeImageMimeType(mimeType))
  return `${String(sequence).padStart(3, '0')}-${stamp}-${taskID}-${imageIndex + 1}.${extension}`
}

async function collectSelectedImageBoardDownloads(): Promise<Array<{ blob: Blob; fileName: string }>> {
  const selectedIDs = selectedImageBoardTaskIds.value
  const selectedTasks = imageBoardTasks.value.filter((task) => selectedIDs.has(task.message.id))
  const files: Array<{ blob: Blob; fileName: string }> = []
  for (const task of selectedTasks) {
    for (const [imageIndex, image] of (task.message.images || []).entries()) {
      const blob = await resolveGeneratedImageBlob(image)
      if (!blob) continue
      files.push({
        blob,
        fileName: imageBoardBatchFileName(task, imageIndex, image.mimeType || blob.type, files.length + 1)
      })
    }
  }
  return files
}

async function downloadSelectedImageBoardTasks(asZip: boolean) {
  if (imageBoardBatchDownloading.value || selectedImageBoardTaskIds.value.size === 0) return
  imageBoardBatchDownloading.value = true
  try {
    const files = await collectSelectedImageBoardDownloads()
    if (files.length === 0) throw new Error(t('playground.imageBoardNoDownloadableImages'))
    if (asZip) {
      const { default: JSZip } = await import('jszip')
      const zip = new JSZip()
      for (const file of files) zip.file(file.fileName, file.blob)
      const archive = await zip.generateAsync({ type: 'blob', compression: 'STORE' })
      const stamp = new Date().toISOString().replace(/\D/g, '').slice(0, 14)
      triggerPlaygroundBlobDownload(archive, `moosecloud-images-${stamp}.zip`)
    } else {
      for (const file of files) {
        triggerPlaygroundBlobDownload(file.blob, file.fileName)
        await sleep(80)
      }
    }
    appStore.showSuccess(t('playground.imageBoardDownloadComplete', { count: files.length }))
  } catch (error) {
    appStore.showError((error as Error)?.message || t('playground.imageBoardDownloadFailed'))
  } finally {
    imageBoardBatchDownloading.value = false
  }
}

function handleImageBoardCardClick(task: PlaygroundImageBoardTask) {
  if (Date.now() < suppressImageBoardCardClickUntil) return
  imageBoardDetailTask.value = task
  imageBoardDetailImageIndex.value = 0
}

function videoBoardSizeLabel(message: PlaygroundMessage): string {
  const video = message.videos?.[0]
  if (video?.width && video.height) return `${video.width}x${video.height}`
  const resolution = message.runRequest?.resolution || ''
  const aspectRatio = message.runRequest?.aspectRatio || ''
  return [resolution, aspectRatio].filter(Boolean).join(' / ') || '-'
}

function videoBoardDurationLabel(message: PlaygroundMessage): string {
  const duration = message.videos?.[0]?.duration ?? message.runRequest?.duration
  return typeof duration === 'number' && duration > 0
    ? t('playground.videoSeconds', { count: duration })
    : '-'
}

function videoBoardTaskSource(task: PlaygroundVideoBoardTask): string {
  const platform = messagePlatform(task.message)
  return platform ? platformLabel(platform) : task.thread.title
}

function videoDownloadProgressPercent(message: PlaygroundMessage): number | null {
  const progress = message.videoDownloadProgress
  return typeof progress === 'number' && Number.isFinite(progress)
    ? Math.min(100, Math.max(0, progress))
    : null
}

function openVideoBoardDetail(task: PlaygroundVideoBoardTask, event: Event) {
  stopVideoBoardPreview(event)
  videoBoardDetailTask.value = task
}

function videoDownloadName(message: PlaygroundMessage, mimeType: string): string {
  const date = new Date(Number.isFinite(message.createdAt) ? message.createdAt : Date.now())
  const stamp = date.toISOString().replace(/\D/g, '').slice(0, 14)
  const normalizedMimeType = mimeType.toLowerCase()
  const extension = normalizedMimeType.includes('webm')
    ? 'webm'
    : normalizedMimeType.includes('quicktime')
      ? 'mov'
      : 'mp4'
  return `sub2api-video-${stamp}.${extension}`
}

function downloadVideoBoardDetail() {
  const task = videoBoardDetailTask.value
  const video = videoBoardDetailVideo.value
  if (!task || !video?.url) return
  const link = document.createElement('a')
  link.href = video.url
  link.download = videoDownloadName(task.message, video.mimeType || 'video/mp4')
  document.body.appendChild(link)
  link.click()
  link.remove()
}

function deleteVideoBoardDetailTask() {
  const task = videoBoardDetailTask.value
  if (!task) return
  closeVideoBoardDetail()
  deleteVideoBoardTask(task)
}

function videoElementFromBoardEvent(event: Event): HTMLVideoElement | null {
  return (event.currentTarget as HTMLElement | null)?.querySelector('video') || null
}

function playVideoBoardPreview(event: Event) {
  const video = videoElementFromBoardEvent(event)
  if (!video) return
  video.muted = true
  if (video.ended) video.currentTime = videoBoardPreviewTime(video)
  void video.play().catch(() => undefined)
}

function stopVideoBoardPreview(event: Event) {
  const video = videoElementFromBoardEvent(event)
  if (!video) return
  video.pause()
  if (video.readyState > 0) video.currentTime = videoBoardPreviewTime(video)
}

function videoBoardPreviewTime(video: HTMLVideoElement): number {
  const stored = Number(video.dataset.previewTime)
  return Number.isFinite(stored) && stored >= 0 ? stored : 0
}

function primeVideoBoardPreview(event: Event) {
  const video = event.currentTarget as HTMLVideoElement | null
  if (!video || !Number.isFinite(video.duration) || video.duration <= 0) return
  const previewTime = Math.min(0.5, Math.max(0.08, video.duration * 0.02))
  video.dataset.previewTime = String(previewTime)
  try {
    video.currentTime = previewTime
  } catch {
    video.dataset.previewTime = '0'
  }
}

function deleteVideoBoardTask(task: PlaygroundVideoBoardTask) {
  deleteMessageFromThread(task.thread, task.message.id)
}

function deleteThread(id: string) {
  const index = threads.value.findIndex((thread) => thread.id === id)
  if (index === -1) return
  const deleted = threads.value[index]
  for (const message of deleted.messages) {
    clearScheduledRunRecovery(message.runId)
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
  for (const thread of threads.value.filter((item) => item.mode === mode.value)) {
    for (const message of thread.messages) clearScheduledRunRecovery(message.runId)
  }
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
          revisedPrompt: image.revisedPrompt,
          width: image.width,
          height: image.height
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
        revisedPrompt: typeof image.revisedPrompt === 'string' ? image.revisedPrompt : undefined,
        width: typeof image.width === 'number' ? image.width : undefined,
        height: typeof image.height === 'number' ? image.height : undefined
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
  mobileHistoryOpen.value = false
  selectDefaultModel()
}

function selectImageWorkspace(nextWorkspace: ImageWorkspaceMode) {
  imageWorkspaceMode.value = nextWorkspace
  nextTick(updateComposerSpacer)
}

function closeComposerPanel() {
  activeComposerPanel.value = null
  composerModelSearch.value = ''
  void nextTick(updateComposerSpacer)
}

function toggleComposerPanel(panel: ComposerPanel) {
  const willOpen = activeComposerPanel.value !== panel
  activeComposerPanel.value = willOpen ? panel : null
  if (panel === 'model') {
    composerModelSearch.value = ''
  }
  void nextTick(updateComposerSpacer)
}

function selectComposerModel(model: string) {
  selectedModel.value = model
  closeComposerPanel()
}

function openImageComposerOptions() {
  if (mode.value !== 'image') {
    selectMode('image')
  }
  toggleComposerPanel('image')
}

function openVideoComposerOptions() {
  if (mode.value !== 'video') {
    selectMode('video')
  }
  toggleComposerPanel('video')
}

function openApiKeyManagement() {
  void router.push({ name: 'Keys' })
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
  if (selectedModel.value && modelAvailable(selectedModel.value)) {
    rememberSelectedModelForMode()
    return
  }
  selectedModel.value = visibleModels.value[0]?.id || ''
  rememberSelectedModelForMode()
}

function selectDefaultPromptOptimizerDraftModel() {
  if (promptOptimizerDraftModel.value && promptOptimizerChatModels.value.some((model) => model.id === promptOptimizerDraftModel.value)) return
  promptOptimizerDraftModel.value = promptOptimizerChatModels.value[0]?.id || ''
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
    customWidth: clampImageDimension(customImageWidth.value, currentImageMaxDimension.value),
    customHeight: clampImageDimension(customImageHeight.value, currentImageMaxDimension.value),
    size: effectiveImageSize.value,
    quality: imageQuality.value,
    outputFormat: outputFormat.value,
    count: Math.min(Math.max(Number(imageCount.value) || 1, 1), 4)
  }
}

function findThreadForMessage(messageId: string): PlaygroundThread | null {
  return threads.value.find((thread) => thread.messages.some((item) => item.id === messageId)) || activeThread.value || null
}

function findImageReuseSourceMessage(message: PlaygroundMessage): PlaygroundMessage | null {
  const thread = findThreadForMessage(message.id)
  if (!thread) return null
  const messageIndex = thread.messages.findIndex((item) => item.id === message.id)
  if (messageIndex <= 0) return null
  for (let index = messageIndex - 1; index >= 0; index -= 1) {
    const candidate = thread.messages[index]
    if (candidate.role === 'user') return candidate
  }
  return null
}

function imageInputToReusableAttachment(image: PlaygroundImageInput, index: number): PlaygroundAttachment {
  const storageId = image.storageId || storedImageIdFromURL(image.dataUrl || '')
  return {
    id: uid('file'),
    name: image.name || `image-${index + 1}.png`,
    type: image.type || '',
    size: 0,
    kind: 'image',
    dataUrl: image.dataUrl,
    storageId: storageId || undefined
  }
}

async function cloneAttachmentForReuse(attachment: PlaygroundAttachment): Promise<PlaygroundAttachment> {
  const clone: PlaygroundAttachment = {
    ...attachment,
    id: uid('file'),
    thumbnailUrl: undefined
  }
  if (clone.kind !== 'image') return clone

  const storageId = clone.storageId || storedImageIdFromURL(clone.dataUrl || '')
  if (storageId) clone.storageId = storageId
  let blob = clone.dataUrl?.startsWith('data:') ? dataURLToBlob(clone.dataUrl) : null

  if (storageId && !blob) {
    const persisted = await loadPlaygroundImageFromDB(storageId).catch((error) => {
      console.warn('Failed to load reusable playground attachment:', error)
      return null
    })
    if (persisted?.blob) {
      blob = persisted.blob
      clone.dataUrl = await blobToDataURL(persisted.blob).catch(() => clone.dataUrl || '')
      clone.type = clone.type || persisted.mimeType || persisted.blob.type
      if (persisted.thumbnailBlob) {
        clone.thumbnailUrl = createTrackedObjectURL(persisted.thumbnailBlob)
      }
    } else if (clone.dataUrl?.startsWith(PLAYGROUND_IMAGE_URL_PREFIX)) {
      clone.dataUrl = undefined
    }
  }

  if (blob && !clone.thumbnailUrl) {
    const thumbnailBlob = await createImageThumbnailBlob(blob).catch(() => blob as Blob)
    clone.thumbnailUrl = createTrackedObjectURL(thumbnailBlob)
    clone.type = clone.type || blob.type
  }
  if (blob && !clone.chatDataUrl) {
    clone.chatDataUrl = await createChatImageDataURL(blob, clone.dataUrl || '').catch(() => clone.dataUrl || '')
  }
  if (!clone.storageId) clone.storageId = uid(`upload-${clone.id}`)
  return clone
}

function isImageDescriptionExpanded(messageId: string): boolean {
  return expandedImageDescriptions.value.has(messageId)
}

function toggleImageDescription(messageId: string) {
  const next = new Set(expandedImageDescriptions.value)
  if (next.has(messageId)) {
    next.delete(messageId)
  } else {
    next.add(messageId)
  }
  expandedImageDescriptions.value = next
}

async function resolveGeneratedImageBlob(image: PlaygroundStoredImageResult): Promise<Blob | null> {
  const storageId = image.storageId || storedImageIdFromURL(image.url || '')
  if (storageId) {
    const persisted = await loadPlaygroundImageFromDB(storageId).catch(() => null)
    if (persisted?.blob) return persisted.blob
    if (persisted?.url) {
      try {
        const response = await fetch(persisted.url)
        if (response.ok) return await response.blob()
      } catch {
        return null
      }
    }
  }
  if (image.url?.startsWith('data:')) return dataURLToBlob(image.url)
  if (image.url && !image.url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX)) {
    try {
      const response = await fetch(image.url)
      if (response.ok) return await response.blob()
    } catch {
      return null
    }
  }
  return null
}

function imageFileExtension(mimeType: string): string {
  if (mimeType === 'image/jpeg') return 'jpg'
  if (mimeType === 'image/webp') return 'webp'
  if (mimeType === 'image/gif') return 'gif'
  return 'png'
}

async function redrawGeneratedImage(message: PlaygroundMessage, image: PlaygroundStoredImageResult, index: number) {
  try {
    const blob = await resolveGeneratedImageBlob(image)
    if (!blob) throw new Error(t('playground.redrawImageFailed'))
    const mimeType = blob.type || image.mimeType || 'image/png'
    const file = new File([blob], `generated-${index + 1}.${imageFileExtension(mimeType)}`, { type: mimeType })
    const attachment = await readFile(file)
    mode.value = 'image'
    if (activeThread.value) {
      activeThread.value.mode = 'image'
      activeThread.value.updatedAt = Date.now()
    }
    if (message.model) selectedModel.value = message.model
    if (message.imageConfig) {
      imageSizeMode.value = message.imageConfig.sizeMode
      imageResolution.value = message.imageConfig.resolution
      imageRatio.value = message.imageConfig.ratio
      customImageWidth.value = message.imageConfig.customWidth
      customImageHeight.value = message.imageConfig.customHeight
      imageQuality.value = message.imageConfig.quality
      outputFormat.value = message.imageConfig.outputFormat
    }
    pendingAttachments.value = [...pendingAttachments.value, attachment]
    await writePlaygroundStateNow()
    appStore.showSuccess(t('playground.redrawImageAdded'))
  } catch (error) {
    appStore.showError((error as Error)?.message || t('playground.redrawImageFailed'))
  }
}

async function reuseImageConfig(message: PlaygroundMessage) {
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
  const sourceMessage = findImageReuseSourceMessage(message)
  const placeholderPrompt = t('playground.attachmentOnlyPrompt')
  const reusablePrompt = typeof message.runRequest?.prompt === 'string'
    ? message.runRequest.prompt
    : (sourceMessage?.content && sourceMessage.content !== placeholderPrompt ? sourceMessage.content : '')
  const sourceAttachments = sourceMessage?.attachments?.length
    ? sourceMessage.attachments
    : (message.runRequest?.images || []).map(imageInputToReusableAttachment)
  const reusableAttachments = await Promise.all(sourceAttachments.map(cloneAttachmentForReuse))
  revokeAttachmentObjectURLs(pendingAttachments.value)
  draftPrompt.value = reusablePrompt
  pendingAttachments.value = reusableAttachments
  if (activeThread.value) {
    activeThread.value.mode = 'image'
    activeThread.value.updatedAt = Date.now()
  }
  await writePlaygroundStateNow()
  appStore.showSuccess(t('playground.imageConfigReused'))
}

function clampImagePreviewZoom(value: number): number {
  if (!Number.isFinite(value)) return imagePreviewZoom.value || 1
  return Math.max(value, 0.02)
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

function handleImagePreviewBackdropClick(event: MouseEvent) {
  if (suppressNextImagePreviewBackdropClick.value) {
    suppressNextImagePreviewBackdropClick.value = false
    return
  }
  if (event.target === imagePreviewViewport.value) {
    closeImagePreview()
  }
}

function handleImagePreviewPointerUp(event: PointerEvent) {
  if (!imagePreviewDragState || imagePreviewDragState.pointerId !== event.pointerId) return
  const moved = Math.hypot(event.clientX - imagePreviewDragState.startX, event.clientY - imagePreviewDragState.startY) > 3
  if (moved) {
    suppressNextImagePreviewBackdropClick.value = true
  }
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
  suppressNextImagePreviewBackdropClick.value = false
}

async function openImagePreview(message: PlaygroundMessage, index: number) {
  const images = message.images || []
  const image = images[index]
  if (!image) return
  const loadToken = ++imagePreviewLoadToken
  imagePreviewZoom.value = 1
  imagePreviewPanX.value = 0
  imagePreviewPanY.value = 0
  imagePreviewNaturalWidth.value = 0
  imagePreviewNaturalHeight.value = 0
  const title = t('playground.generatedImageAlt', { n: index + 1 })
  const storageId = image.storageId || storedImageIdFromURL(image.url)
  let previewUrl = ''
  let ownsObjectUrl = false
  let revisedPrompt = image.revisedPrompt
  let mimeType = normalizeImageMimeType(image.mimeType || '') || 'image/png'
  if (storageId) {
    const persisted = await loadPlaygroundImageFromDB(storageId).catch((error) => {
      console.warn('Failed to load original playground image:', error)
      return null
    })
    if (persisted?.blob) {
      previewUrl = createTrackedObjectURL(persisted.blob)
      ownsObjectUrl = true
      revisedPrompt = revisedPrompt || persisted.revisedPrompt
      mimeType = normalizeImageMimeType(persisted.blob.type || persisted.mimeType || mimeType)
    } else if (persisted?.url) {
      previewUrl = persisted.url
      revisedPrompt = revisedPrompt || persisted.revisedPrompt
      mimeType = normalizeImageMimeType(persisted.mimeType || mimeType)
    }
  }
  if (!previewUrl && image.url && !image.url.startsWith(PLAYGROUND_IMAGE_URL_PREFIX)) {
    previewUrl = image.url
  }
  if (loadToken !== imagePreviewLoadToken) {
    if (ownsObjectUrl) revokeTrackedObjectURL(previewUrl)
    return
  }
  if (!previewUrl) {
    appStore.showError(t('playground.imageCacheMissing'))
    return
  }
  if (imagePreview.value?.ownsObjectUrl && imagePreview.value.url !== previewUrl) {
    revokeTrackedObjectURL(imagePreview.value?.url)
  }
  imagePreview.value = {
    url: previewUrl,
    title,
    downloadName: imagePreviewDownloadName(message.createdAt, index, mimeType),
    mimeType,
    revisedPrompt,
    messageId: message.id,
    index,
    total: images.length,
    ownsObjectUrl
  }
  await nextTick()
  imagePreviewViewport.value?.focus({ preventScroll: true })
}

async function openAttachmentImagePreview(attachment: PlaygroundAttachment) {
  closeImagePreview()
  const loadToken = imagePreviewLoadToken
  let previewUrl = ''
  let mimeType = normalizeImageMimeType(attachment.type) || 'image/png'
  let ownsObjectUrl = false
  try {
    const original = await resolveAttachmentBlob(attachment)
    if (original) {
      previewUrl = createTrackedObjectURL(original)
      mimeType = normalizeImageMimeType(original.type || attachment.type) || mimeType
      ownsObjectUrl = true
    }
  } catch {
    // Fall back to the rendered thumbnail when an older attachment has no persisted original.
  }
  if (!previewUrl) {
    previewUrl = attachment.dataUrl?.startsWith('data:') ? attachment.dataUrl : (attachment.thumbnailUrl || '')
    mimeType = normalizeImageMimeType(attachment.type || dataURLToBlob(previewUrl)?.type || '') || mimeType
  }
  if (!previewUrl) {
    appStore.showError(t('playground.imageCacheMissing'))
    return
  }
  if (loadToken !== imagePreviewLoadToken) {
    if (ownsObjectUrl) revokeTrackedObjectURL(previewUrl)
    return
  }
  imagePreview.value = {
    url: previewUrl,
    title: attachment.name,
    downloadName: attachment.name || imagePreviewDownloadName(Date.now(), 0, mimeType),
    mimeType,
    messageId: '',
    index: 0,
    total: 1,
    ownsObjectUrl
  }
  await nextTick()
  imagePreviewViewport.value?.focus({ preventScroll: true })
}

function imagePreviewDownloadName(createdAt: number, index: number, mimeType: string): string {
  const date = new Date(Number.isFinite(createdAt) ? createdAt : Date.now())
  const stamp = date.toISOString().replace(/\D/g, '').slice(0, 14)
  return `moosecloud-image-${stamp}-${index + 1}.${imageFileExtension(normalizeImageMimeType(mimeType))}`
}

async function navigateImagePreview(direction: -1 | 1) {
  const current = imagePreview.value
  if (!current?.messageId || current.total <= 1) return
  const thread = findThreadForMessage(current.messageId)
  const message = thread?.messages.find((item) => item.id === current.messageId)
  if (!message?.images?.length) return
  const nextIndex = wrapGalleryIndex(current.index, direction, message.images.length)
  await openImagePreview(message, nextIndex)
}

function attachmentPreviewUrl(attachment: PlaygroundAttachment): string {
  return attachment.thumbnailUrl || (attachment.dataUrl?.startsWith('data:') ? attachment.dataUrl : '')
}

async function resolveAttachmentBlob(attachment: PlaygroundAttachment): Promise<Blob | null> {
  if (attachment.dataUrl?.startsWith('data:')) {
    const blob = dataURLToBlob(attachment.dataUrl)
    if (blob) return blob
  }
  const storageId = attachment.storageId || storedImageIdFromURL(attachment.dataUrl || '')
  if (!storageId) return null
  const persisted = await loadPlaygroundImageFromDB(storageId).catch(() => null)
  return persisted?.blob || null
}

function isPlaygroundImageStorageReferenced(storageId: string): boolean {
  return threads.value.some((thread) => {
    const attachmentUsesStorage = (attachment: PlaygroundAttachment) => {
      return (attachment.storageId || storedImageIdFromURL(attachment.dataUrl || '')) === storageId
    }
    if (thread.pendingAttachments.some(attachmentUsesStorage)) return true
    return thread.messages.some((message) => {
      return message.attachments?.some(attachmentUsesStorage) ||
        message.images?.some((image) => (image.storageId || storedImageIdFromURL(image.url)) === storageId) ||
        message.runRequest?.images?.some((image) => {
          return (image.storageId || storedImageIdFromURL(image.dataUrl || '')) === storageId
        })
    })
  })
}

async function openDoodleEditor(attachment: PlaygroundAttachment) {
  try {
    const blob = await resolveAttachmentBlob(attachment)
    if (!blob) throw new Error(t('playground.doodleLoadFailed'))
    const image = await loadImageElement(blob)
    doodleSourceImage = image
    doodleStrokes.value = []
    doodleRedoStrokes.value = []
    doodleTool.value = 'brush'
    doodleBrushOpacity.value = 0.5
    doodleEditorFullscreen.value = false
    doodleStickerResizing.value = false
    doodleStickerPanelOpen.value = false
    doodleEditor.value = {
      attachmentId: attachment.id,
      name: attachment.name,
      naturalWidth: image.naturalWidth,
      naturalHeight: image.naturalHeight
    }
    await nextTick()
    const canvas = doodleCanvas.value
    if (!canvas) throw new Error(t('playground.doodleLoadFailed'))
    const ratio = Math.min(1, PLAYGROUND_DOODLE_MAX_DISPLAY_EDGE / Math.max(image.naturalWidth, image.naturalHeight))
    canvas.width = Math.max(1, Math.round(image.naturalWidth * ratio))
    canvas.height = Math.max(1, Math.round(image.naturalHeight * ratio))
    doodleCanvasWidth.value = canvas.width
    doodleCanvasHeight.value = canvas.height
    doodleOverlayCanvas = document.createElement('canvas')
    doodleOverlayCanvas.width = canvas.width
    doodleOverlayCanvas.height = canvas.height
    renderDoodleCanvas()
    await nextTick()
    updateDoodleCanvasViewportSize()
    doodleResizeObserver?.disconnect()
    if (doodleCanvasViewport.value) {
      doodleResizeObserver = new ResizeObserver(updateDoodleCanvasViewportSize)
      doodleResizeObserver.observe(doodleCanvasViewport.value)
    }
    fitDoodleCanvas()
  } catch (error) {
    closeDoodleEditor()
    appStore.showError((error as Error)?.message || t('playground.doodleLoadFailed'))
  }
}

function closeDoodleEditor(force = false) {
  if (doodleSaving.value && !force) return
  if (doodleRenderFrame !== null) {
    cancelAnimationFrame(doodleRenderFrame)
    doodleRenderFrame = null
  }
  doodlePointerId = null
  doodleStickerDragState = null
  doodleStickerResizeState = null
  doodleCanvasPanState = null
  doodleCanvasPanning.value = false
  doodleCtrlPanning.value = false
  doodleStickerResizing.value = false
  doodleResizeObserver?.disconnect()
  doodleResizeObserver = null
  doodleSourceImage = null
  doodleOverlayCanvas = null
  doodleEditorFullscreen.value = false
  doodleEditor.value = null
  doodleStrokes.value = []
  doodleRedoStrokes.value = []
  doodleCanvasWidth.value = 0
  doodleCanvasHeight.value = 0
  doodleCanvasViewportWidth.value = 0
  doodleCanvasViewportHeight.value = 0
  doodleZoom.value = 1
  doodlePanX.value = 0
  doodlePanY.value = 0
  doodleStickerPanelOpen.value = false
  selectedDoodleStickerId.value = null
  doodleSaving.value = false
}

async function toggleDoodleEditorFullscreen() {
  if (doodleSaving.value) return
  doodleEditorFullscreen.value = !doodleEditorFullscreen.value
  await nextTick()
  updateDoodleCanvasViewportSize()
  clampDoodleCanvasPan()
}

function selectDoodleColor(color: string) {
  doodleColor.value = color
  doodleTool.value = 'brush'
}

function clampDoodleZoom(value: number): number {
  if (!Number.isFinite(value)) return doodleZoom.value || 1
  return Math.max(value, 0.02)
}

function adjustDoodleZoom(delta: number) {
  doodleZoom.value = clampDoodleZoom(doodleZoom.value + delta)
  clampDoodleCanvasPan()
}

function doodleCanvasPanLimit() {
  const overflowX = Math.max(0, (doodleCanvasDisplaySize.value.width - doodleCanvasViewportWidth.value) / 2)
  const overflowY = Math.max(0, (doodleCanvasDisplaySize.value.height - doodleCanvasViewportHeight.value) / 2)
  return {
    x: overflowX,
    y: overflowY
  }
}

function clampDoodleCanvasPan() {
  const limit = doodleCanvasPanLimit()
  doodlePanX.value = Math.min(Math.max(doodlePanX.value, -limit.x), limit.x)
  doodlePanY.value = Math.min(Math.max(doodlePanY.value, -limit.y), limit.y)
}

function updateDoodleCanvasViewportSize() {
  const viewport = doodleCanvasViewport.value
  if (!viewport) return
  doodleCanvasViewportWidth.value = viewport.clientWidth
  doodleCanvasViewportHeight.value = viewport.clientHeight
  clampDoodleCanvasPan()
}

function fitDoodleCanvas() {
  doodleZoom.value = 1
  doodlePanX.value = 0
  doodlePanY.value = 0
  doodleCanvasPanning.value = false
}

function toggleDoodleStickerPanel() {
  doodleTool.value = 'sticker'
  doodleStickerPanelOpen.value = !doodleStickerPanelOpen.value
}

function selectDoodleSticker(sticker: string) {
  doodleSelectedSticker.value = sticker
  doodleTool.value = 'sticker'
  doodleStickerPanelOpen.value = true
}

function drawDoodleOperation(context: CanvasRenderingContext2D, operation: DoodleOperation) {
  context.save()
  if (operation.kind === 'sticker') {
    context.font = `${operation.size}px "Segoe UI Emoji", "Apple Color Emoji", sans-serif`
    context.textAlign = 'center'
    context.textBaseline = 'middle'
    context.fillText(operation.emoji, operation.point.x, operation.point.y)
    context.restore()
    return
  }
  if (operation.points.length === 0) {
    context.restore()
    return
  }
  context.globalCompositeOperation = operation.tool === 'eraser' ? 'destination-out' : 'source-over'
  context.globalAlpha = operation.tool === 'brush' ? operation.opacity : 1
  context.strokeStyle = operation.color
  context.fillStyle = operation.color
  context.lineWidth = operation.size
  context.lineCap = 'round'
  context.lineJoin = 'round'
  if (operation.points.length === 1) {
    const point = operation.points[0]
    context.beginPath()
    context.arc(point.x, point.y, operation.size / 2, 0, Math.PI * 2)
    context.fill()
  } else {
    context.beginPath()
    context.moveTo(operation.points[0].x, operation.points[0].y)
    for (const point of operation.points.slice(1)) {
      context.lineTo(point.x, point.y)
    }
    context.stroke()
  }
  context.restore()
}

function selectedDoodleSticker(): DoodleSticker | null {
  const selectedId = selectedDoodleStickerId.value
  if (!selectedId) return null
  const operation = doodleStrokes.value.find((item) => item.kind === 'sticker' && item.id === selectedId)
  return operation?.kind === 'sticker' ? operation : null
}

function findDoodleStickerAtPoint(point: DoodlePoint): DoodleSticker | null {
  for (let index = doodleStrokes.value.length - 1; index >= 0; index -= 1) {
    const operation = doodleStrokes.value[index]
    if (operation.kind !== 'sticker') continue
    const halfSize = operation.size / 2
    if (Math.abs(point.x - operation.point.x) <= halfSize && Math.abs(point.y - operation.point.y) <= halfSize) {
      return operation
    }
  }
  return null
}

function doodleStickerResizeHandlePoint(sticker: DoodleSticker): DoodlePoint {
  const halfSize = sticker.size / 2
  return {
    x: sticker.point.x + halfSize,
    y: sticker.point.y + halfSize
  }
}

function doodleStickerResizeHandleRadius(): number {
  const canvas = doodleCanvas.value
  if (!canvas) return 12
  const rect = canvas.getBoundingClientRect()
  if (!rect.width) return 12
  return Math.max(8, 8 * canvas.width / rect.width)
}

function isDoodleStickerResizeHandle(point: DoodlePoint, sticker: DoodleSticker): boolean {
  const handle = doodleStickerResizeHandlePoint(sticker)
  return Math.hypot(point.x - handle.x, point.y - handle.y) <= doodleStickerResizeHandleRadius() * 1.8
}

function drawDoodleStickerSelection(context: CanvasRenderingContext2D) {
  const sticker = selectedDoodleSticker()
  if (!sticker) return
  const halfSize = sticker.size / 2
  const handle = doodleStickerResizeHandlePoint(sticker)
  const handleRadius = doodleStickerResizeHandleRadius()
  context.save()
  context.strokeStyle = '#0ea5e9'
  context.lineWidth = Math.max(1, sticker.size / 32)
  context.setLineDash([context.lineWidth * 3, context.lineWidth * 2])
  context.strokeRect(sticker.point.x - halfSize, sticker.point.y - halfSize, sticker.size, sticker.size)
  context.setLineDash([])
  context.beginPath()
  context.arc(handle.x, handle.y, handleRadius, 0, Math.PI * 2)
  context.fillStyle = '#0ea5e9'
  context.fill()
  context.strokeStyle = '#ffffff'
  context.lineWidth = Math.max(1.5, handleRadius / 4)
  context.stroke()
  context.restore()
}

function renderDoodleCanvas() {
  const canvas = doodleCanvas.value
  const overlay = doodleOverlayCanvas
  const image = doodleSourceImage
  if (!canvas || !overlay || !image) return
  const overlayContext = overlay.getContext('2d')
  const context = canvas.getContext('2d')
  if (!overlayContext || !context) return
  overlayContext.clearRect(0, 0, overlay.width, overlay.height)
  for (const operation of doodleStrokes.value) drawDoodleOperation(overlayContext, operation)
  context.clearRect(0, 0, canvas.width, canvas.height)
  context.drawImage(image, 0, 0, canvas.width, canvas.height)
  context.drawImage(overlay, 0, 0)
  drawDoodleStickerSelection(context)
}

function scheduleDoodleRender() {
  if (doodleRenderFrame !== null) return
  doodleRenderFrame = requestAnimationFrame(() => {
    doodleRenderFrame = null
    renderDoodleCanvas()
  })
}

function doodlePointFromEvent(event: PointerEvent): DoodlePoint | null {
  const canvas = doodleCanvas.value
  if (!canvas) return null
  const rect = canvas.getBoundingClientRect()
  return mapClientPointToCanvas(event.clientX, event.clientY, rect, canvas.width, canvas.height)
}

function handleDoodleCanvasWheel(event: WheelEvent) {
  if (doodleSaving.value) return
  const viewport = doodleCanvasViewport.value
  if (!viewport) return
  const previousZoom = doodleZoom.value
  const zoomFactor = event.deltaY < 0 ? 1.12 : 1 / 1.12
  const nextZoom = clampDoodleZoom(Number((previousZoom * zoomFactor).toFixed(3)))
  if (nextZoom === previousZoom) return

  const rect = viewport.getBoundingClientRect()
  const pointerX = event.clientX - rect.left - rect.width / 2
  const pointerY = event.clientY - rect.top - rect.height / 2
  const scaleRatio = nextZoom / previousZoom
  doodlePanX.value = pointerX - (pointerX - doodlePanX.value) * scaleRatio
  doodlePanY.value = pointerY - (pointerY - doodlePanY.value) * scaleRatio
  doodleZoom.value = nextZoom
  clampDoodleCanvasPan()
}

function startDoodleCanvasPan(event: PointerEvent) {
  const canvas = doodleCanvas.value
  const viewport = doodleCanvasViewport.value
  if (!canvas || !viewport) return
  event.preventDefault()
  canvas.setPointerCapture(event.pointerId)
  doodleCanvasPanState = {
    pointerId: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    panX: doodlePanX.value,
    panY: doodlePanY.value
  }
  doodleCanvasPanning.value = true
}

function handleDoodlePointerDown(event: PointerEvent) {
  if (doodleSaving.value || (event.button !== 0 && event.button !== 1) || doodlePointerId !== null || doodleStickerDragState || doodleStickerResizeState || doodleCanvasPanState) return
  if (event.button === 1 || doodleTool.value === 'pan' || doodleCtrlPanning.value || event.ctrlKey) {
    startDoodleCanvasPan(event)
    return
  }
  const canvas = doodleCanvas.value
  const point = doodlePointFromEvent(event)
  if (!canvas || !point) return
  event.preventDefault()
  if (doodleTool.value === 'sticker') {
    const selectedSticker = selectedDoodleSticker()
    if (selectedSticker && isDoodleStickerResizeHandle(point, selectedSticker)) {
      canvas.setPointerCapture(event.pointerId)
      doodleStickerResizeState = {
        pointerId: event.pointerId,
        stickerId: selectedSticker.id,
        startSize: selectedSticker.size,
        startDistance: Math.max(1, Math.hypot(point.x - selectedSticker.point.x, point.y - selectedSticker.point.y))
      }
      doodleStickerResizing.value = true
      return
    }
    const existingSticker = findDoodleStickerAtPoint(point)
    if (existingSticker) {
      selectedDoodleStickerId.value = existingSticker.id
      canvas.setPointerCapture(event.pointerId)
      doodleStickerDragState = {
        pointerId: event.pointerId,
        stickerId: existingSticker.id,
        offsetX: point.x - existingSticker.point.x,
        offsetY: point.y - existingSticker.point.y
      }
      scheduleDoodleRender()
      return
    }
    doodleRedoStrokes.value = []
    const sticker: DoodleSticker = {
      kind: 'sticker',
      id: uid('doodle-sticker'),
      emoji: doodleSelectedSticker.value,
      size: Math.min(160, Math.max(40, doodleBrushSize.value * 5)),
      point
    }
    doodleStrokes.value = [...doodleStrokes.value, {
      ...sticker
    }]
    selectedDoodleStickerId.value = sticker.id
    scheduleDoodleRender()
    return
  }
  selectedDoodleStickerId.value = null
  doodleRedoStrokes.value = []
  canvas.setPointerCapture(event.pointerId)
  doodlePointerId = event.pointerId
  const tool: DoodleDrawingTool = doodleTool.value === 'eraser' ? 'eraser' : 'brush'
  doodleStrokes.value = [...doodleStrokes.value, {
    kind: 'stroke',
    tool,
    color: doodleColor.value,
    size: doodleBrushSize.value,
    opacity: doodleBrushOpacity.value,
    points: [point]
  }]
  scheduleDoodleRender()
}

function handleDoodlePointerMove(event: PointerEvent) {
  const canvasPan = doodleCanvasPanState
  if (canvasPan?.pointerId === event.pointerId) {
    doodlePanX.value = canvasPan.panX + event.clientX - canvasPan.startX
    doodlePanY.value = canvasPan.panY + event.clientY - canvasPan.startY
    clampDoodleCanvasPan()
    return
  }
  const stickerResize = doodleStickerResizeState
  if (stickerResize?.pointerId === event.pointerId) {
    const point = doodlePointFromEvent(event)
    const sticker = doodleStrokes.value.find((item) => item.kind === 'sticker' && item.id === stickerResize.stickerId)
    if (!point || !sticker || sticker.kind !== 'sticker') return
    const distance = Math.hypot(point.x - sticker.point.x, point.y - sticker.point.y)
    sticker.size = Math.min(
      PLAYGROUND_DOODLE_STICKER_MAX_SIZE,
      Math.max(PLAYGROUND_DOODLE_STICKER_MIN_SIZE, stickerResize.startSize * distance / stickerResize.startDistance)
    )
    scheduleDoodleRender()
    return
  }
  const stickerDrag = doodleStickerDragState
  if (stickerDrag?.pointerId === event.pointerId) {
    const point = doodlePointFromEvent(event)
    const sticker = doodleStrokes.value.find((item) => item.kind === 'sticker' && item.id === stickerDrag.stickerId)
    if (!point || !sticker || sticker.kind !== 'sticker') return
    sticker.point = {
      x: point.x - stickerDrag.offsetX,
      y: point.y - stickerDrag.offsetY
    }
    scheduleDoodleRender()
    return
  }
  if (doodlePointerId !== event.pointerId) return
  const point = doodlePointFromEvent(event)
  const operation = doodleStrokes.value[doodleStrokes.value.length - 1]
  if (!point || !operation || operation.kind !== 'stroke') return
  operation.points.push(point)
  scheduleDoodleRender()
}

function handleDoodlePointerUp(event: PointerEvent) {
  const canvasPan = doodleCanvasPanState
  if (canvasPan?.pointerId === event.pointerId) {
    const canvas = doodleCanvas.value
    if (canvas?.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
    doodleCanvasPanState = null
    doodleCanvasPanning.value = false
    return
  }
  const stickerResize = doodleStickerResizeState
  if (stickerResize?.pointerId === event.pointerId) {
    const canvas = doodleCanvas.value
    if (canvas?.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
    doodleStickerResizeState = null
    doodleStickerResizing.value = false
    scheduleDoodleRender()
    return
  }
  const stickerDrag = doodleStickerDragState
  if (stickerDrag?.pointerId === event.pointerId) {
    const canvas = doodleCanvas.value
    if (canvas?.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
    doodleStickerDragState = null
    scheduleDoodleRender()
    return
  }
  if (doodlePointerId !== event.pointerId) return
  const canvas = doodleCanvas.value
  if (canvas?.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
  doodlePointerId = null
  scheduleDoodleRender()
}

function undoDoodleStroke() {
  const stroke = doodleStrokes.value[doodleStrokes.value.length - 1]
  if (!stroke) return
  doodleStrokes.value = doodleStrokes.value.slice(0, -1)
  doodleRedoStrokes.value = [...doodleRedoStrokes.value, stroke]
  if (stroke.kind === 'sticker' && stroke.id === selectedDoodleStickerId.value) selectedDoodleStickerId.value = null
  scheduleDoodleRender()
}

function redoDoodleStroke() {
  const stroke = doodleRedoStrokes.value[doodleRedoStrokes.value.length - 1]
  if (!stroke) return
  doodleRedoStrokes.value = doodleRedoStrokes.value.slice(0, -1)
  doodleStrokes.value = [...doodleStrokes.value, stroke]
  if (stroke.kind === 'sticker') selectedDoodleStickerId.value = stroke.id
  scheduleDoodleRender()
}

function clearDoodleStrokes() {
  doodleStrokes.value = []
  doodleRedoStrokes.value = []
  selectedDoodleStickerId.value = null
  scheduleDoodleRender()
}

function deleteSelectedDoodleSticker() {
  const selectedId = selectedDoodleStickerId.value
  if (!selectedId) return
  doodleStrokes.value = doodleStrokes.value.filter((operation) => operation.kind !== 'sticker' || operation.id !== selectedId)
  doodleRedoStrokes.value = []
  selectedDoodleStickerId.value = null
  scheduleDoodleRender()
}

function canvasToPNGBlob(canvas: HTMLCanvasElement): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error(t('playground.doodleSaveFailed'))), 'image/png')
  })
}

async function saveDoodleAttachment() {
  const editor = doodleEditor.value
  const sourceImage = doodleSourceImage
  const overlay = doodleOverlayCanvas
  if (!editor || !sourceImage || !overlay || doodleSaving.value) return
  let originalAttachment: PlaygroundAttachment | null = null
  let replacementAttachment: PlaygroundAttachment | null = null
  doodleSaving.value = true
  try {
    renderDoodleCanvas()
    const output = document.createElement('canvas')
    output.width = editor.naturalWidth
    output.height = editor.naturalHeight
    const context = output.getContext('2d')
    if (!context) throw new Error(t('playground.doodleSaveFailed'))
    context.drawImage(sourceImage, 0, 0, output.width, output.height)
    context.drawImage(overlay, 0, 0, output.width, output.height)
    const blob = await canvasToPNGBlob(output)
    const baseName = editor.name.replace(/\.[^.]+$/, '') || 'image'
    const replacement = await readFile(new File([blob], `${baseName}-doodle.png`, { type: 'image/png' }))
    const oldAttachment = pendingAttachments.value.find((attachment) => attachment.id === editor.attachmentId)
    if (!oldAttachment) throw new Error(t('playground.doodleSaveFailed'))
    const oldStorageId = oldAttachment.storageId
    replacement.id = editor.attachmentId
    originalAttachment = oldAttachment
    replacementAttachment = replacement
    pendingAttachments.value = pendingAttachments.value.map((attachment) => attachment.id === editor.attachmentId ? replacement : attachment)
    await writePlaygroundStateNow(true)
    revokeAttachmentObjectURLs([oldAttachment])
    if (oldStorageId && !isPlaygroundImageStorageReferenced(oldStorageId)) {
      try {
        await deletePlaygroundImageFromDB(oldStorageId)
        persistedAttachmentStorageIds.delete(oldStorageId)
      } catch (error) {
        console.warn('Failed to clean up replaced playground image:', error)
      }
    }
    closeDoodleEditor(true)
    appStore.showSuccess(t('playground.doodleSaved'))
  } catch (error) {
    if (originalAttachment && replacementAttachment) {
      const currentAttachment = pendingAttachments.value.find((attachment) => attachment.id === editor.attachmentId)
      const isCurrentReplacement = currentAttachment?.storageId === replacementAttachment.storageId
      if (isCurrentReplacement) {
        pendingAttachments.value = pendingAttachments.value.map((attachment) => {
          return attachment.id === editor.attachmentId ? originalAttachment as PlaygroundAttachment : attachment
        })
        revokeAttachmentObjectURLs([replacementAttachment])
        const replacementStorageId = replacementAttachment.storageId
        if (replacementStorageId && !isPlaygroundImageStorageReferenced(replacementStorageId)) {
          try {
            await deletePlaygroundImageFromDB(replacementStorageId)
            persistedAttachmentStorageIds.delete(replacementStorageId)
          } catch (cleanupError) {
            console.warn('Failed to clean up uncommitted playground image:', cleanupError)
          }
        }
        persistPlaygroundState()
      }
    }
    doodleSaving.value = false
    appStore.showError((error as Error)?.message || t('playground.doodleSaveFailed'))
  }
}

function closeImagePreview() {
  imagePreviewLoadToken += 1
  if (imagePreview.value?.ownsObjectUrl) {
    revokeTrackedObjectURL(imagePreview.value.url)
  }
  imagePreview.value = null
  resetImagePreviewView()
  imagePreviewNaturalWidth.value = 0
  imagePreviewNaturalHeight.value = 0
}

function handlePlaygroundGlobalKeydown(event: KeyboardEvent) {
  if (activeComposerPanel.value && event.key === 'Escape') {
    event.preventDefault()
    closeComposerPanel()
    return
  }

  if (doodleEditor.value) {
    if (event.key === 'Control') {
      doodleCtrlPanning.value = true
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      if (doodleEditorFullscreen.value) {
        void toggleDoodleEditorFullscreen()
      } else {
        closeDoodleEditor()
      }
      return
    }
    if (selectedDoodleStickerId.value && (event.key === 'Delete' || event.key === 'Backspace')) {
      event.preventDefault()
      deleteSelectedDoodleSticker()
      return
    }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
      event.preventDefault()
      if (doodleSaving.value) return
      if (event.shiftKey) redoDoodleStroke()
      else undoDoodleStroke()
    }
    return
  }
  if (imageBoardDetailTask.value) {
    if (event.key === 'Escape') {
      event.preventDefault()
      closeImageBoardDetail()
    }
    return
  }
  if (videoBoardDetailTask.value) {
    if (event.key === 'Escape') {
      event.preventDefault()
      closeVideoBoardDetail()
    }
    return
  }
  if (event.key === 'Escape' && selectedImageBoardTaskIds.value.size > 0) {
    event.preventDefault()
    clearImageBoardSelection()
    return
  }
  if (!imagePreview.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    closeImagePreview()
  } else if (event.key === 'ArrowLeft') {
    event.preventDefault()
    void navigateImagePreview(-1)
  } else if (event.key === 'ArrowRight') {
    event.preventDefault()
    void navigateImagePreview(1)
  }
}

function handlePlaygroundGlobalKeyup(event: KeyboardEvent) {
  if (event.key === 'Control') {
    doodleCtrlPanning.value = false
  }
}

function deleteMessage(messageId: string) {
  const thread = activeThread.value
  if (!thread) return
  deleteMessageFromThread(thread, messageId)
}

function deleteMessageFromThread(thread: PlaygroundThread, messageId: string) {
  const deleted = thread.messages.find((message) => message.id === messageId)
  clearScheduledRunRecovery(deleted?.runId)
  revokeImageObjectURLs(deleted?.images)
  revokeVideoObjectURLs(deleted?.videos)
  revokeAttachmentObjectURLs(deleted?.attachments)
  thread.messages = thread.messages.filter((message) => message.id !== messageId)
  if (expandedImageDescriptions.value.has(messageId)) {
    const next = new Set(expandedImageDescriptions.value)
    next.delete(messageId)
    expandedImageDescriptions.value = next
  }
  if (imageBoardDetailTask.value?.message.id === messageId) {
    closeImageBoardDetail()
  }
  if (videoBoardDetailTask.value?.message.id === messageId) {
    closeVideoBoardDetail()
  }
  if (selectedImageBoardTaskIds.value.has(messageId)) {
    const next = new Set(selectedImageBoardTaskIds.value)
    next.delete(messageId)
    selectedImageBoardTaskIds.value = next
  }
  thread.updatedAt = Date.now()
  syncThreadRunning(thread)
}

function updateImageBoardSelection(clientX: number, clientY: number) {
  const state = imageBoardSelectionState
  const root = imageBoardRoot.value
  if (!state || !root) return
  const left = Math.min(state.startX, clientX)
  const right = Math.max(state.startX, clientX)
  const top = Math.min(state.startY, clientY)
  const bottom = Math.max(state.startY, clientY)
  const next = new Set(state.initialSelectedIds)
  const initiallySelected = state.initialSelectedIds
  const cards = root.querySelectorAll<HTMLElement>('[data-image-board-task-id]')
  for (const card of cards) {
    const taskId = card.dataset.imageBoardTaskId
    if (!taskId) continue
    const task = imageBoardTasks.value.find((item) => item.message.id === taskId)
    if (!task || task.message.pending) continue
    const rect = card.getBoundingClientRect()
    const intersects = imageBoardRectsIntersect(
      { left, top, right, bottom },
      { left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom }
    )
    if (intersects) {
      if (initiallySelected.has(taskId)) next.delete(taskId)
      else next.add(taskId)
    } else if (!initiallySelected.has(taskId)) {
      next.delete(taskId)
    }
  }
  selectedImageBoardTaskIds.value = next
  imageBoardSelection.value = {
    startX: state.startX,
    startY: state.startY,
    currentX: clientX,
    currentY: clientY
  }
}

function cleanupImageBoardPointerSelection() {
  imageBoardSelectionState = null
  imageBoardSelection.value = null
  document.body.classList.remove('select-none')
  window.removeEventListener('pointermove', handleImageBoardPointerMove)
  window.removeEventListener('pointerup', handleImageBoardPointerUp)
  window.removeEventListener('pointercancel', handleImageBoardPointerUp)
}

function handleImageBoardPointerDown(event: PointerEvent) {
  if (event.button !== 0 || !(event.target instanceof Element)) return
  if (event.target.closest('button, a, input, textarea, select')) return
  const root = imageBoardRoot.value
  if (!root || !root.contains(event.target)) return
  const card = event.target.closest<HTMLElement>('[data-image-board-task-id]')
  const taskId = card?.dataset.imageBoardTaskId || null
  const task = taskId ? imageBoardTasks.value.find((item) => item.message.id === taskId) : null
  const additive = event.ctrlKey || event.metaKey
  imageBoardSelectionState = {
    pointerId: event.pointerId,
    taskId: task?.message.pending ? null : taskId,
    additive,
    initialSelectedIds: new Set(selectedImageBoardTaskIds.value),
    startX: event.clientX,
    startY: event.clientY,
    didDrag: false
  }
  window.addEventListener('pointermove', handleImageBoardPointerMove)
  window.addEventListener('pointerup', handleImageBoardPointerUp)
  window.addEventListener('pointercancel', handleImageBoardPointerUp)
}

function handleImageBoardPointerMove(event: PointerEvent) {
  const state = imageBoardSelectionState
  if (!state || state.pointerId !== event.pointerId) return
  if (!state.didDrag && Math.hypot(event.clientX - state.startX, event.clientY - state.startY) < 6) return
  state.didDrag = true
  document.body.classList.add('select-none')
  event.preventDefault()
  updateImageBoardSelection(event.clientX, event.clientY)
}

function handleImageBoardPointerUp(event: PointerEvent) {
  const state = imageBoardSelectionState
  if (!state || state.pointerId !== event.pointerId) return
  if (state.didDrag) {
    suppressImageBoardCardClickUntil = Date.now() + 250
  } else if (state.additive && state.taskId) {
    toggleImageBoardTaskSelection(state.taskId)
    suppressImageBoardCardClickUntil = Date.now() + 250
  } else if (!state.taskId) {
    clearImageBoardSelection()
  }
  cleanupImageBoardPointerSelection()
}

async function scrollMessagesToBottom() {
  await nextTick()
  if (messageScroller.value) {
    if (isBoardMode.value) {
      messageScroller.value.scrollTop = 0
      return
    }
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

function autoResizeComposerInput() {
  void nextTick(() => {
    const textarea = composerTextarea.value
    if (!textarea) return
    if (!textarea.value.trim()) {
      composerInputHeightCustomized.value = false
      composerManualInputHeight.value = PLAYGROUND_COMPOSER_MIN_HEIGHT
    }
    const minimum = composerInputHeightCustomized.value
      ? composerManualInputHeight.value
      : PLAYGROUND_COMPOSER_MIN_HEIGHT
    // `h-full` otherwise makes scrollHeight reflect the previously expanded container.
    const inlineHeight = textarea.style.height
    textarea.style.height = '0px'
    const contentHeight = textarea.scrollHeight + 24
    textarea.style.height = inlineHeight
    composerInputHeight.value = clampComposerInputHeight(Math.max(minimum, contentHeight))
    updateComposerSpacer()
  })
}

function handleComposerInputResizePointerDown(event: PointerEvent) {
  if (event.button !== 0) return
  event.preventDefault()
  if (event.currentTarget instanceof HTMLElement) {
    event.currentTarget.setPointerCapture(event.pointerId)
  }
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
  const height = clampComposerInputHeight(composerInputResizeState.height + delta)
  composerInputHeightCustomized.value = true
  composerManualInputHeight.value = height
  composerInputHeight.value = height
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

function ensureActiveKeySelection() {
  const selectedKeyIsActive = activeKeys.value.some((key) => String(key.id) === selectedKeyId.value)
  if (!selectedKeyIsActive) {
    selectedKeyId.value = activeKeys.value.length > 0 ? String(activeKeys.value[0].id) : ''
  }
  const promptOptimizerKeyWasSelected = Boolean(promptOptimizerKeyId.value)
  const promptOptimizerKeyIsActive = activeKeys.value.some((key) => String(key.id) === promptOptimizerKeyId.value)
  if (!promptOptimizerKeyIsActive) {
    promptOptimizerKeyId.value = selectedKeyId.value || (activeKeys.value[0] ? String(activeKeys.value[0].id) : '')
    if (promptOptimizerKeyWasSelected) promptOptimizerModel.value = ''
  }
}

async function loadKeys() {
  loadingKeys.value = true
  try {
    // Load all user keys and filter locally. Status filters are not applied
    // consistently to every platform by older upgraded backend instances.
    const response = await keysAPI.list(1, 1000)
    apiKeys.value = response.items
    ensureActiveKeySelection()
  } catch (error) {
    appStore.showError((error as Error)?.message || t('playground.loadKeysFailed'))
  } finally {
    loadingKeys.value = false
  }
}

async function loadModels() {
  if (!selectedKey.value) {
    models.value = []
    selectedModel.value = ''
    return
  }
  modelAbortController?.abort()
  const controller = new AbortController()
  modelAbortController = controller
  loadingModels.value = true
  modelLoadError.value = ''
  try {
    const fetched = await fetchModels(selectedKey.value.key, undefined, controller.signal)
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

async function loadPromptOptimizerModels(keyId = promptOptimizerDraftKeyId.value) {
  const optimizerKey = activeKeys.value.find((key) => String(key.id) === keyId)
  promptOptimizerModelAbortController?.abort()
  promptOptimizerModels.value = []
  promptOptimizerModelLoadError.value = ''
  if (!optimizerKey) {
    promptOptimizerDraftModel.value = ''
    loadingPromptOptimizerModels.value = false
    return
  }

  const controller = new AbortController()
  promptOptimizerModelAbortController = controller
  loadingPromptOptimizerModels.value = true
  try {
    const fetched = await fetchModels(optimizerKey.key, undefined, controller.signal)
    if (controller.signal.aborted || promptOptimizerDraftKeyId.value !== keyId) return
    promptOptimizerModels.value = fetched
    selectDefaultPromptOptimizerDraftModel()
  } catch (error) {
    if (controller.signal.aborted) return
    promptOptimizerModelLoadError.value = (error as Error)?.message || t('playground.loadModelsFailed')
    promptOptimizerDraftModel.value = ''
  } finally {
    if (promptOptimizerModelAbortController === controller) {
      promptOptimizerModelAbortController = null
      loadingPromptOptimizerModels.value = false
    }
  }
}

function validateRun(): boolean {
  if (!selectedKey.value) {
    appStore.showInfo(t('playground.selectKeyFirst'))
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
  if (!selectedPromptOptimizerKey.value) {
    appStore.showInfo(t('playground.selectOptimizerGroupFirst'))
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

function playgroundRequestStatus(error: unknown): number {
  if (!error || typeof error !== 'object') return 0
  const status = Number((error as Record<string, unknown>).status)
  return Number.isFinite(status) ? status : 0
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

function clearScheduledRunRecovery(runId?: string, resetAttempts = true) {
  if (!runId) return
  const timer = runRecoveryTimers.get(runId)
  if (timer !== undefined) window.clearTimeout(timer)
  runRecoveryTimers.delete(runId)
  if (resetAttempts) runRecoveryAttempts.delete(runId)
}

function scheduleRunRecovery(thread: PlaygroundThread, message: PlaygroundMessage) {
  const runId = message.runId
  if (!runId || !playgroundViewMounted || runRecoveryTimers.has(runId)) return
  const attempt = (runRecoveryAttempts.get(runId) || 0) + 1
  if (attempt > PLAYGROUND_RUN_RECOVERY_MAX_ATTEMPTS) {
    void finalizeUnrecoverablePlaygroundRun(thread, message)
    return
  }
  runRecoveryAttempts.set(runId, attempt)
  const delay = playgroundRetryDelayMs(
    attempt,
    PLAYGROUND_RUN_RECOVERY_BASE_MS,
    PLAYGROUND_RUN_RECOVERY_MAX_MS
  )
  const timer = window.setTimeout(() => {
    runRecoveryTimers.delete(runId)
    if (!playgroundViewMounted) return
    const currentThread = threads.value.find((item) => item.id === thread.id)
    const currentMessage = currentThread?.messages.find((item) => item.id === message.id)
    if (!currentThread || !currentMessage?.pending || currentMessage.runId !== runId) return
    void resumePlaygroundRun(currentThread, currentMessage)
  }, delay)
  runRecoveryTimers.set(runId, timer)
}

async function finalizeUnrecoverablePlaygroundRun(thread: PlaygroundThread, message: PlaygroundMessage) {
  clearScheduledRunRecovery(message.runId)
  message.pending = false
  message.progress = ''
  message.error = true
  message.content = t('playground.requestInterrupted')
  thread.lastRunError = message.content
  thread.updatedAt = Date.now()
  if (message.imageConfig) {
    await persistCompletedImageMessage(thread, message)
  } else {
    await writePlaygroundStateNow()
  }
  syncThreadRunning(thread)
}

async function preserveRecoverableRun(
  thread: PlaygroundThread,
  message: PlaygroundMessage
) {
  message.pending = true
  message.error = false
  message.progress = message.imageConfig ? t('playground.generatingImages') : t('playground.waiting')
  thread.lastRunError = ''
  thread.updatedAt = Date.now()
  if (message.imageConfig) {
    await persistCompletedImageMessage(thread, message)
  } else {
    await writePlaygroundStateNow()
  }
  scheduleRunRecovery(thread, message)
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
  attachments: PlaygroundAttachment[],
  context: PlaygroundRunContext
): PlaygroundRunRequest {
  return {
    id: runId,
    mode: 'image',
    apiKey: context.apiKey,
    platform: context.platform,
    model: context.model,
    prompt,
    size: context.imageSize,
    n: context.imageCount,
    quality: context.imageQuality,
    outputFormat: context.outputFormat,
    images: buildImageEditInputs(attachments)
  }
}

function buildImageEditInputs(attachments: PlaygroundAttachment[]): PlaygroundImageInput[] {
  return attachments
    .filter((attachment) => attachment.kind === 'image' && typeof attachment.dataUrl === 'string' && attachment.dataUrl.startsWith('data:'))
    .map((attachment) => ({
      name: attachment.name || 'image.png',
      type: attachment.type,
      dataUrl: attachment.dataUrl || '',
      storageId: attachment.storageId
    }))
}

function stripRunAPIKey(request: PlaygroundRunRequest): PlaygroundRestorableRunRequest {
  const { apiKey: _apiKey, ...rest } = request
  return rest
}

async function ensureBackendPlaygroundRun(
  request: PlaygroundRunRequest,
  controller: AbortController
): Promise<PlaygroundRun> {
  let lastError: unknown
  for (let attempt = 1; attempt <= PLAYGROUND_RUN_START_MAX_ATTEMPTS; attempt += 1) {
    try {
      return await startPlaygroundRun(request, controller.signal)
    } catch (error) {
      if (controller.signal.aborted) throw error
      if (!isRetryablePlaygroundRequestError(error)) throw error
      lastError = error

      if (request.id) {
        try {
          return await getPlaygroundRun(request.id, controller.signal)
        } catch (lookupError) {
          if (controller.signal.aborted) throw lookupError
          const lookupStatus = playgroundRequestStatus(lookupError)
          if (lookupStatus !== 404 && !isRetryablePlaygroundRequestError(lookupError)) {
            throw lookupError
          }
        }
      }

      if (attempt < PLAYGROUND_RUN_START_MAX_ATTEMPTS) {
        await sleep(playgroundRetryDelayMs(attempt), controller.signal)
      }
    }
  }
  throw markRecoverablePlaygroundError(lastError, t('playground.runTimeout'))
}

async function pollPlaygroundRun(
  runId: string,
  apply: (run: PlaygroundRun) => Promise<void> | void,
  controller: AbortController
): Promise<PlaygroundRun> {
  const startedAt = Date.now()
  let consecutiveFailures = 0
  let notFoundFailures = 0
  while (true) {
    if (controller.signal.aborted) throw new DOMException('Aborted', 'AbortError')
    let run: PlaygroundRun
    try {
      run = await getPlaygroundRun(runId, controller.signal)
      consecutiveFailures = 0
      notFoundFailures = 0
    } catch (error) {
      if (controller.signal.aborted) throw error
      const status = playgroundRequestStatus(error)
      if (status === 404) notFoundFailures += 1
      if (status === 404 && notFoundFailures > PLAYGROUND_RUN_NOT_FOUND_MAX_ATTEMPTS) {
        throw markRecoverablePlaygroundError(error, t('playground.runTimeout'))
      }
      const retryable = isRetryablePlaygroundRequestError(error) ||
        (status === 404 && notFoundFailures <= PLAYGROUND_RUN_NOT_FOUND_MAX_ATTEMPTS)
      if (!retryable) {
        throw error
      }
      consecutiveFailures += 1
      if (Date.now() - startedAt > PLAYGROUND_RUN_POLL_MAX_MS) {
        throw markRecoverablePlaygroundError(error, t('playground.runTimeout'))
      }
      await sleep(playgroundRetryDelayMs(consecutiveFailures), controller.signal)
      continue
    }
    await apply(run)
    if (isTerminalPlaygroundRun(run)) return run
    if (Date.now() - startedAt > PLAYGROUND_RUN_POLL_MAX_MS) {
      throw markRecoverablePlaygroundError(new Error(t('playground.runTimeout')), t('playground.runTimeout'))
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
    clearScheduledRunRecovery(message.runId)
    message.pending = false
    message.progress = ''
    message.error = true
    message.content = run.error || t('playground.runFailed')
    thread.lastRunError = message.content
    thread.updatedAt = Date.now()
    await writePlaygroundStateNow()
    return
  }
  clearScheduledRunRecovery(message.runId)
  message.pending = false
  message.progress = ''
  message.error = false
  message.content = run.content || message.content || t('playground.emptyTextResponse')
  message.raw = run.raw
  message.durationMs = run.durationMs
  thread.updatedAt = Date.now()
  await writePlaygroundStateNow()
}

async function hydratePlaygroundRunImages(run: PlaygroundRun, signal: AbortSignal): Promise<PlaygroundImageResult[]> {
  return Promise.all((run.images || []).map(async (image, index) => {
    const assetIndex = Number.isInteger(image.assetIndex) ? Number(image.assetIndex) : index
    let blob: Blob | null = null
    let lastError: unknown
    for (let attempt = 1; attempt <= PLAYGROUND_IMAGE_FETCH_MAX_ATTEMPTS; attempt += 1) {
      try {
        blob = await getPlaygroundRunImage(run.id, assetIndex, signal)
        break
      } catch (error) {
        if (signal.aborted) throw error
        const retryable = isRetryablePlaygroundRequestError(error) || playgroundRequestStatus(error) === 404
        if (!retryable) {
          throw error
        }
        lastError = error
        if (attempt >= PLAYGROUND_IMAGE_FETCH_MAX_ATTEMPTS) {
          throw markRecoverablePlaygroundError(error, t('playground.runTimeout'))
        }
        await sleep(playgroundRetryDelayMs(attempt), signal)
      }
    }
    if (!blob) {
      throw markRecoverablePlaygroundError(lastError, t('playground.noImageReturned'))
    }
    const mimeType = image.mimeType || blob.type || 'image/png'
    const normalizedBlob = blob.type ? blob : new Blob([blob], { type: mimeType })
    let dataUrl: string
    try {
      dataUrl = await blobToDataURL(normalizedBlob)
    } catch (error) {
      throw markRecoverablePlaygroundError(error, t('playground.noImageReturned'))
    }
    return {
      url: dataUrl,
      revisedPrompt: image.revisedPrompt,
      mimeType,
      width: image.width,
      height: image.height
    }
  }))
}

async function applyCompletedImageRun(
  thread: PlaygroundThread,
  message: PlaygroundMessage,
  run: PlaygroundRun,
  signal: AbortSignal
) {
  if (run.status !== 'succeeded') {
    clearScheduledRunRecovery(message.runId)
    message.pending = false
    message.progress = ''
    message.error = true
    message.content = run.error || t('playground.runFailed')
    thread.lastRunError = message.content
    thread.updatedAt = Date.now()
    await persistCompletedImageMessage(thread, message)
    return
  }
  clearScheduledRunRecovery(message.runId)
  message.pending = false
  message.progress = ''
  message.error = false
  message.durationMs = run.durationMs
  message.content = t('playground.imageGenerated')
  thread.updatedAt = Date.now()
  syncThreadRunning(thread)

  try {
    const images = await hydratePlaygroundRunImages(run, signal)
    message.content = images.length > 0 ? t('playground.imageGenerated') : t('playground.noImageReturned')
    message.images = images.map((image, index) => ({
      ...image,
      storageId: uid(`image-${message.id}-${index}`),
      mimeType: image.mimeType || (image.url.startsWith('data:')
        ? image.url.match(/^data:([^;,]+)/)?.[1]
        : undefined)
    }))
    message.raw = undefined
  } catch (error) {
    message.error = true
    message.content = (error as Error)?.message || t('playground.noImageReturned')
    thread.lastRunError = message.content
  }
  await persistCompletedImageMessage(thread, message)
}

async function resumePlaygroundRun(thread: PlaygroundThread, message: PlaygroundMessage) {
  if (!message.pending || !message.runId || runAbortControllers.has(message.runId)) return
  const controller = new AbortController()
  const runMode: PlaygroundMode = message.imageConfig ? 'image' : thread.mode
  runAbortControllers.set(message.runId, { controller, mode: runMode, threadId: thread.id, runId: message.runId })
  thread.running = true
  try {
    const request = message.runRequest
    const apiKey = apiKeyForRunMessage(message)
    if (request && apiKey && !message.runStarted) {
      await ensureBackendPlaygroundRun({ ...request, apiKey } as PlaygroundRunRequest, controller)
      message.runStarted = true
      thread.updatedAt = Date.now()
      if (message.imageConfig) {
        await persistCompletedImageMessage(thread, message)
      } else {
        await writePlaygroundStateNow()
      }
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
      await applyCompletedImageRun(thread, message, finalRun, controller.signal)
    } else if (finalRun.mode === 'video') {
      await applyCompletedVideoRun(thread, message, finalRun, controller.signal)
    } else {
      await applyCompletedChatRun(thread, message, finalRun)
    }
    if (activeThreadId.value !== thread.id) {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
  } catch (error) {
    if (controller.signal.aborted) return
    if (isRecoverablePlaygroundError(error)) {
      await preserveRecoverableRun(thread, message)
      return
    }
    clearScheduledRunRecovery(message.runId)
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
    if (runAbortControllers.get(message.runId)?.controller === controller) {
      runAbortControllers.delete(message.runId)
      syncThreadRunning(thread)
    }
  }
}

function resumePendingPlaygroundRuns() {
  for (const thread of threads.value) {
    const pendingMessages = thread.messages.filter((message) => message.pending && message.runId)
    if (pendingMessages.length === 0) {
      thread.running = false
      continue
    }
    thread.running = true
    for (const pendingMessage of pendingMessages) {
      void resumePlaygroundRun(thread, pendingMessage)
    }
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
  const optimizerKey = selectedPromptOptimizerKey.value
  if (optimizingPrompt.value || !validatePromptOptimizer() || !optimizerKey) return
  promptOptimizeAbortController?.abort()
  const controller = new AbortController()
  promptOptimizeAbortController = controller
  optimizingPrompt.value = true
  const originalPrompt = draftPrompt.value.trim()
  let optimized = ''
  try {
    const response = await streamChatCompletion({
      apiKey: optimizerKey.key,
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

function attachmentChatImageUrl(attachment: PlaygroundAttachment): string {
  const chatDataUrl = attachment.chatDataUrl?.startsWith('data:')
    ? normalizeImageDataURLHeader(attachment.chatDataUrl)
    : ''
  const dataUrl = attachment.dataUrl?.startsWith('data:')
    ? normalizeImageDataURLHeader(attachment.dataUrl)
    : ''
  if (isPreferredChatImageDataURL(chatDataUrl)) return chatDataUrl
  if (isPreferredChatImageDataURL(dataUrl)) return dataUrl
  return chatDataUrl || dataUrl
}

function buildUserMessageContent(prompt: string, attachments: PlaygroundAttachment[]): PlaygroundChatMessage['content'] {
  const imageAttachments = attachments
    .filter((attachment) => attachment.kind === 'image')
    .map((attachment) => ({ attachment, url: attachmentChatImageUrl(attachment) }))
    .filter((item) => item.url.startsWith('data:'))
  const text = `${prompt}${attachmentPrompt(attachments)}`.trim()
  if (imageAttachments.length === 0) return text
  return [
    { type: 'text', text: text || t('playground.attachedImagesOnly') },
    ...imageAttachments.map(({ url }) => ({
      type: 'image_url' as const,
      image_url: { url }
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
  if ((thread.mode !== 'image' && thread.mode !== 'video' && thread.running) || !validateRun() || !selectedKey.value) return
  lastRunError.value = ''
  const context: PlaygroundRunContext = {
    mode: thread.mode,
    keyId: selectedKeyId.value,
    apiKey: selectedKey.value.key,
    model: effectiveModel.value,
    platform: selectedKeyPlatform.value || platformForModel(effectiveModel.value),
    temperature: temperature.value,
    topP: topP.value,
    maxTokens: maxTokens.value,
    presencePenalty: presencePenalty.value,
    frequencyPenalty: frequencyPenalty.value,
    imageSize: effectiveImageSize.value,
    imageCount: Math.min(Math.max(Number(imageCount.value) || 1, 1), 4),
    imageQuality: imageQuality.value,
    outputFormat: outputFormat.value,
    videoDuration: videoDuration.value,
    videoResolution: videoResolution.value,
    videoAspectRatio: videoAspectRatio.value
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
  runAbortControllers.set(runId, { controller, mode: context.mode, threadId: thread.id, runId })
  thread.running = true
  await scrollMessagesToBottom()

  try {
    if (context.mode === 'image') {
      await runImageGeneration(thread, prompt, attachments, context, imageConfig, controller, runId)
    } else if (context.mode === 'video') {
      await runVideoGeneration(thread, prompt, attachments, context, controller, runId)
    } else {
      await runStreamingChat(thread, prompt, attachments, context, controller, runId)
    }
    if (activeThreadId.value === thread.id) {
      appStore.showSuccess(t('playground.runSuccess'))
    }
  } catch (error) {
    if (controller.signal.aborted) return
    if (isRecoverablePlaygroundError(error)) return
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
    if (runAbortControllers.get(runId)?.controller === controller) {
      runAbortControllers.delete(runId)
      syncThreadRunning(thread)
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

  try {
    await ensureBackendPlaygroundRun(runRequest, controller)
    assistantMessage.runStarted = true
    await writePlaygroundStateNow()
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
      clearScheduledRunRecovery(assistantMessage.runId)
      assistantMessage.pending = false
      assistantMessage.progress = ''
      assistantMessage.error = true
      assistantMessage.content = finalRun.error || t('playground.runFailed')
      thread.lastRunError = assistantMessage.content
      thread.updatedAt = Date.now()
      await writePlaygroundStateNow()
      throw markRunErrorHandled(buildRunError(finalRun), assistantMessage.content)
    }
    clearScheduledRunRecovery(assistantMessage.runId)
    assistantMessage.content = assistantMessage.content || finalRun.content || t('playground.emptyTextResponse')
    assistantMessage.raw = finalRun.raw
    assistantMessage.pending = false
    assistantMessage.progress = ''
    assistantMessage.error = false
    assistantMessage.durationMs = finalRun.durationMs
    thread.updatedAt = Date.now()
    if (activeThreadId.value !== thread.id) {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
    await writePlaygroundStateNow()
  } catch (error) {
    if (controller.signal.aborted) throw error
    if (isHandledRunError(error)) throw error
    if (isRecoverablePlaygroundError(error)) {
      await preserveRecoverableRun(thread, assistantMessage)
      throw markRunErrorHandled(error, (error as Error).message)
    }
    clearScheduledRunRecovery(assistantMessage.runId)
    assistantMessage.pending = false
    assistantMessage.progress = ''
    assistantMessage.error = true
    assistantMessage.content = (error as Error)?.message || t('playground.runFailed')
    thread.lastRunError = assistantMessage.content
    thread.updatedAt = Date.now()
    await writePlaygroundStateNow()
    throw markRunErrorHandled(error, assistantMessage.content)
  }
}

async function runImageGeneration(
  thread: PlaygroundThread,
  prompt: string,
  attachments: PlaygroundAttachment[],
  context: PlaygroundRunContext,
  imageConfig: PlaygroundImageConfig,
  controller: AbortController,
  runId: string
) {
  const runRequest = buildImageRunRequest(runId, prompt, attachments, context)
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
    assistantMessage.runStarted = true
    await writePlaygroundStateNow()
    const response = await pollPlaygroundRun(runId, async (run) => {
      assistantMessage.progress = run.status === 'queued' ? t('playground.waiting') : t('playground.generatingImages')
      thread.updatedAt = Date.now()
      await writePlaygroundStateNow()
    }, controller)
    if (response.status !== 'succeeded') {
      throw buildRunError(response)
    }
    await applyCompletedImageRun(thread, assistantMessage, response, controller.signal)
    if (activeThreadId.value !== thread.id) {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
  } catch (error) {
    if (controller.signal.aborted) throw error
    if (isRecoverablePlaygroundError(error)) {
      await preserveRecoverableRun(thread, assistantMessage)
      throw markRunErrorHandled(error, (error as Error).message)
    }
    clearScheduledRunRecovery(assistantMessage.runId)
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

async function runVideoGeneration(
  thread: PlaygroundThread,
  prompt: string,
  attachments: PlaygroundAttachment[],
  context: PlaygroundRunContext,
  controller: AbortController,
  runId: string
) {
  const runRequest: PlaygroundRunRequest = {
    id: runId,
    mode: 'video',
    apiKey: context.apiKey,
    platform: context.platform,
    model: context.model,
    prompt,
    n: 1,
    images: buildImageEditInputs(attachments).slice(0, 1),
    duration: context.videoDuration,
    resolution: context.videoResolution,
    aspectRatio: context.videoAspectRatio
  }
  const assistantMessage: PlaygroundMessage = {
    id: uid('msg'),
    role: 'assistant',
    content: '',
    createdAt: Date.now(),
    model: context.model,
    platform: context.platform,
    progress: t('playground.generatingVideo'),
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
    assistantMessage.runStarted = true
    await writePlaygroundStateNow()
    const response = await pollPlaygroundRun(runId, async (run) => {
      assistantMessage.progress = run.status === 'queued' ? t('playground.waiting') : t('playground.generatingVideo')
      thread.updatedAt = Date.now()
      await writePlaygroundStateNow()
    }, controller)
    if (response.status !== 'succeeded') {
      throw buildRunError(response)
    }
    await applyCompletedVideoRun(thread, assistantMessage, response, controller.signal)
    if (activeThreadId.value !== thread.id) {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
  } catch (error) {
    if (controller.signal.aborted) throw error
    if (isRecoverablePlaygroundError(error)) {
      await preserveRecoverableRun(thread, assistantMessage)
      throw markRunErrorHandled(error, (error as Error).message)
    }
    clearScheduledRunRecovery(assistantMessage.runId)
    assistantMessage.pending = false
    assistantMessage.progress = ''
    assistantMessage.error = true
    assistantMessage.content = (error as Error)?.message || t('playground.runFailed')
    thread.lastRunError = assistantMessage.content
    thread.updatedAt = Date.now()
    if (activeThreadId.value !== thread.id) {
      thread.unreadCount = (thread.unreadCount || 0) + 1
    }
    await writePlaygroundStateNow()
    throw markRunErrorHandled(error, assistantMessage.content)
  }
}

async function applyCompletedVideoRun(
  thread: PlaygroundThread,
  message: PlaygroundMessage,
  run: PlaygroundRun,
  signal: AbortSignal
) {
  if (run.status !== 'succeeded') {
    clearScheduledRunRecovery(message.runId)
    message.pending = false
    message.progress = ''
    message.videoDownloadProgress = undefined
    message.error = true
    message.content = run.error || t('playground.runFailed')
    thread.lastRunError = message.content
    thread.updatedAt = Date.now()
    await writePlaygroundStateNow()
    return
  }
  clearScheduledRunRecovery(message.runId)
  message.pending = false
  message.progress = t('playground.videoDownloading')
  message.videoDownloadProgress = null
  message.error = false
  message.videoRestoreUnavailable = false
  message.content = t('playground.videoDownloading')
  thread.lastRunError = ''
  message.videos = (run.videos || []).map((video, index) => ({
    ...video,
    url: '',
    thumbnailUrl: undefined,
    assetIndex: Number.isInteger(video.assetIndex) ? video.assetIndex : index
  }))
  message.raw = undefined
  message.durationMs = run.durationMs
  thread.updatedAt = Date.now()
  await writePlaygroundStateNow()
  try {
    const videos = await hydratePlaygroundRunVideos(run, signal, (progress) => {
      message.videoDownloadProgress = progress
      message.progress = progress === null
        ? t('playground.videoDownloading')
        : t('playground.videoDownloadingProgress', { progress: Math.round(progress) })
    })
    if (videos.length === 0) throw new Error(t('playground.videoCacheMissing'))
    message.videos = videos
    message.pending = false
    message.progress = ''
    message.videoDownloadProgress = undefined
    message.content = t('playground.videoGenerated')
    message.videoRestoreUnavailable = false
  } catch (error) {
    message.pending = false
    message.progress = ''
    message.videoDownloadProgress = undefined
    if (signal.aborted) {
      message.content = t('playground.requestStopped')
    } else {
      console.warn('Failed to hydrate completed playground video:', error)
      message.error = true
      message.content = t('playground.videoCacheMissing')
      thread.lastRunError = message.content
    }
  }
  thread.updatedAt = Date.now()
  syncThreadRunning(thread)
  await writePlaygroundStateNow()
}

async function hydratePlaygroundRunVideos(
  run: PlaygroundRun,
  signal: AbortSignal,
  onProgress?: (progress: number | null) => void,
  options: { retry?: boolean; allowRemote?: boolean } = {},
): Promise<PlaygroundVideoResult[]> {
  return Promise.all((run.videos || []).map(async (video, index) => {
    const assetIndex = Number.isInteger(video.assetIndex) ? Number(video.assetIndex) : index
    let blob: Blob | null = await loadVideoBlobFromDirectory(video.localFileName)
    let lastError: unknown
    for (let attempt = 1; options.allowRemote !== false && !blob && attempt <= PLAYGROUND_IMAGE_FETCH_MAX_ATTEMPTS; attempt += 1) {
      try {
        blob = await getPlaygroundRunVideo(run.id, assetIndex, signal, (progress) => {
          onProgress?.(typeof progress.percent === 'number' ? progress.percent : null)
        })
        break
      } catch (error) {
        if (signal.aborted) throw error
        const retryable = isRetryablePlaygroundRequestError(error) || playgroundRequestStatus(error) === 404
        if (!retryable || options.retry === false) throw error
        lastError = error
        if (attempt >= PLAYGROUND_IMAGE_FETCH_MAX_ATTEMPTS) {
          throw markRecoverablePlaygroundError(error, t('playground.runTimeout'))
        }
        await sleep(playgroundRetryDelayMs(attempt), signal)
      }
    }
    if (!blob) {
      throw markRecoverablePlaygroundError(lastError, t('playground.runTimeout'))
    }
    const mimeType = video.mimeType || blob.type || 'video/mp4'
    const normalizedBlob = blob.type ? blob : new Blob([blob], { type: mimeType })
    const fileName = video.localFileName || videoLocalFileName(run.id, assetIndex, mimeType)
    let localFileName = video.localFileName
    if (!localFileName) {
      try {
        if (await saveVideoBlobToDirectory(normalizedBlob, fileName)) localFileName = fileName
      } catch (error) {
        console.warn('Failed to persist generated video to selected directory:', error)
      }
    }
    return {
      ...video,
      url: createTrackedObjectURL(normalizedBlob),
      thumbnailUrl: undefined,
      mimeType,
      assetIndex,
      localFileName
    }
  }))
}

function stopRun() {
  const thread = activeThread.value
  if (!thread) return
  for (const [key, handle] of runAbortControllers) {
    if (handle.threadId !== thread.id) continue
    handle.controller.abort()
    if (handle.runId) {
      void cancelPlaygroundRun(handle.runId).catch(() => undefined)
    }
    runAbortControllers.delete(key)
  }
  for (const pendingMessage of thread.messages.filter((message) => message.pending)) {
    clearScheduledRunRecovery(pendingMessage.runId)
    if (pendingMessage.runId) {
      void cancelPlaygroundRun(pendingMessage.runId).catch(() => undefined)
    }
    pendingMessage.pending = false
    pendingMessage.progress = ''
    if (!pendingMessage.content) pendingMessage.content = t('playground.requestStopped')
  }
  syncThreadRunning(thread)
}

function syncThreadRunning(thread: PlaygroundThread) {
  thread.running = thread.messages.some((message) => message.pending)
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
    const storageId = isImage ? uid(`upload-${id}`) : undefined
    const reader = new FileReader()

    reader.onload = async () => {
      let thumbnailUrl: string | undefined
      let chatDataUrl: string | undefined
      const dataUrl = normalizeImageDataURLHeader(String(reader.result || ''))
      if (isImage && storageId) {
        const [thumbnailBlob, chatBlob] = await Promise.all([
          createImageThumbnailBlob(file).catch(() => file),
          createChatImageBlob(file).catch(() => file)
        ])
        attachmentThumbnailBlobCache.set(storageId, thumbnailBlob)
        thumbnailUrl = createTrackedObjectURL(thumbnailBlob)
        chatDataUrl = chatBlob === file
          ? dataUrl
          : await blobToDataURL(chatBlob).catch(() => dataUrl)
      }
      resolve({
        id,
        name: file.name,
        type: file.type,
        size: file.size,
        kind: isImage ? 'image' : isText ? 'text' : 'file',
        dataUrl: isImage ? dataUrl : undefined,
        chatDataUrl,
        storageId,
        thumbnailUrl,
        text: isText ? dataUrl.slice(0, 12000) : undefined
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
  await addAttachmentFiles(files)
  input.value = ''
}

async function addAttachmentFiles(files: File[]) {
  if (files.length === 0) return
  const acceptedFiles = mode.value === 'image'
    ? files.filter((file) => file.type.startsWith('image/'))
    : files
  if (acceptedFiles.length !== files.length) {
    appStore.showError(t('playground.imageFilesOnly'))
  }
  if (acceptedFiles.length === 0) return
  const attachments = await Promise.all(acceptedFiles.map(readFile))
  pendingAttachments.value = [...pendingAttachments.value, ...attachments]
  persistPlaygroundState()
}

function handleComposerDragEnter(event: DragEvent) {
  if (!Array.from(event.dataTransfer?.types || []).includes('Files')) return
  composerDragDepth += 1
  composerDragActive.value = true
}

function handleComposerDragOver(event: DragEvent) {
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function handleComposerDragLeave() {
  composerDragDepth = Math.max(0, composerDragDepth - 1)
  if (composerDragDepth === 0) composerDragActive.value = false
}

async function handleComposerDrop(event: DragEvent) {
  composerDragDepth = 0
  composerDragActive.value = false
  await addAttachmentFiles(Array.from(event.dataTransfer?.files || []))
}

async function handleComposerPaste(event: ClipboardEvent) {
  const pastedImages = Array.from(event.clipboardData?.items || [])
    .filter((item) => item.kind === 'file' && item.type.startsWith('image/'))
    .map((item) => item.getAsFile())
    .filter((file): file is File => Boolean(file))
    .map((file, index) => ensureClipboardImageName(file, index))
  if (pastedImages.length === 0) return
  event.preventDefault()
  await addAttachmentFiles(pastedImages)
}

function ensureClipboardImageName(file: File, index: number): File {
  if (file.name && file.name !== 'image.png') return file
  const extension = file.type === 'image/jpeg'
    ? 'jpg'
    : file.type === 'image/webp'
      ? 'webp'
      : file.type === 'image/gif'
        ? 'gif'
        : 'png'
  return new File([file], `pasted-image-${Date.now()}-${index + 1}.${extension}`, { type: file.type })
}

function removeAttachment(id: string) {
  const removed = pendingAttachments.value.find((attachment) => attachment.id === id)
  revokeAttachmentObjectURLs(removed ? [removed] : undefined)
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

watch(selectedKeyId, () => {
  if (restoringState) return
  models.value = []
  selectedModel.value = ''
  selectedModelsByMode.value = {}
  modelLoadError.value = ''
  lastRunError.value = ''
  composerModelSearch.value = ''
  loadModels()
})

watch(mode, (nextMode) => {
  if (activeComposerPanel.value === 'image' || activeComposerPanel.value === 'video') {
    closeComposerPanel()
  }
  if (nextMode !== 'image') showImageSizeModal.value = false
  selectDefaultModel()
})

watch(promptOptimizerDraftKeyId, (keyId, previousKeyId) => {
  if (!showPromptOptimizerModal.value || keyId === previousKeyId) return
  promptOptimizerDraftModel.value = ''
  void loadPromptOptimizerModels(keyId)
}, { flush: 'sync' })

watch(draftPrompt, autoResizeComposerInput)

watch(selectedModel, (value) => {
  if (restoringState) return
  if (!value.trim()) return
  rememberSelectedModelForMode()
})

watch([effectiveModel, imageRatio, imageResolution, customImageWidth, customImageHeight], ([model, ratio, resolution]) => {
  if (!isGptImage2Model(model)) return
  if (gptImage2SizeFor('1K', ratio) === null) {
    imageRatio.value = '1:1'
  } else if (gptImage2SizeFor(resolution, ratio) === null) {
    imageResolution.value = '2K'
  }
  customImageWidth.value = clampImageDimension(customImageWidth.value, GPT_IMAGE_2_MAX_DIMENSION)
  customImageHeight.value = clampImageDimension(customImageHeight.value, GPT_IMAGE_2_MAX_DIMENSION)
}, { immediate: true })

watch(() => ({
  activeThreadId: activeThreadId.value,
  mode: mode.value,
  selectedKeyId: selectedKeyId.value,
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
  imageWorkspaceMode: imageWorkspaceMode.value,
  videoDuration: videoDuration.value,
  videoResolution: videoResolution.value,
  videoAspectRatio: videoAspectRatio.value,
  promptOptimizerKeyId: promptOptimizerKeyId.value,
  promptOptimizerModel: promptOptimizerModel.value,
  showComposerConfig: showComposerConfig.value,
  composerInputHeight: composerManualInputHeight.value,
  composerInputHeightCustomized: composerInputHeightCustomized.value,
  threads: threads.value
}), persistPlaygroundState, { deep: true })

watch([
  pendingAttachments,
  playgroundNotice,
  () => showComposerConfig.value,
  () => composerInputHeight.value,
  () => mode.value
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
  const publicSettingsPromise = appStore.fetchPublicSettings()
  await loadKeys()
  await restorePlaygroundState(false)
  ensureActiveKeySelection()
  if (threads.value.length === 0) {
    createThread('chat')
  }
  await publicSettingsPromise
  await loadModels()
  selectDefaultModel()
  persistenceReady = true
  resumePendingPlaygroundRuns()
  await writePlaygroundStateNow()
  updateComposerSpacer()
  autoResizeComposerInput()
  if (composerDock.value) {
    composerResizeObserver = new ResizeObserver(updateComposerSpacer)
    composerResizeObserver.observe(composerDock.value)
  }
  window.addEventListener(PLAYGROUND_STATE_UPDATED_EVENT, handlePlaygroundStateUpdated)
  window.addEventListener('keydown', handlePlaygroundGlobalKeydown)
  window.addEventListener('keyup', handlePlaygroundGlobalKeyup)
  scrollMessagesToBottom()
  const videoHydrationController = new AbortController()
  persistedVideoHydrationController = videoHydrationController
  void hydratePersistedVideos(videoHydrationController.signal)
    .then(() => writePlaygroundStateNow())
    .catch((error) => console.warn('Failed to hydrate persisted playground videos:', error))
    .finally(() => {
      if (persistedVideoHydrationController === videoHydrationController) {
        persistedVideoHydrationController = null
      }
    })
})

onBeforeUnmount(() => {
  playgroundViewMounted = false
  cleanupImageBoardPointerSelection()
  closeDoodleEditor(true)
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
  window.removeEventListener('keydown', handlePlaygroundGlobalKeydown)
  window.removeEventListener('keyup', handlePlaygroundGlobalKeyup)
  composerInputResizeState = null
  modelAbortController?.abort()
  promptOptimizerModelAbortController?.abort()
  promptOptimizeAbortController?.abort()
  persistedVideoHydrationController?.abort()
  persistedVideoHydrationController = null
  for (const handle of runAbortControllers.values()) {
    handle.controller.abort()
  }
  for (const threadId of runAbortControllers.keys()) {
    runAbortControllers.delete(threadId)
  }
  for (const timer of runRecoveryTimers.values()) window.clearTimeout(timer)
  runRecoveryTimers.clear()
  runRecoveryAttempts.clear()
  void writePlaygroundStateNow()
})
</script>

<style scoped>
.playground-composer-shell {
  container-name: playground-composer;
  container-type: inline-size;
}

.composer-control-panel {
  margin-bottom: 0.75rem;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  background: rgb(255 255 255);
  box-shadow: 0 14px 30px -24px rgb(15 23 42 / 0.42);
}

.composer-model-picker {
  display: grid;
  max-height: min(30rem, calc(100dvh - 8rem));
  grid-template-columns: 18rem minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
}

.composer-model-key {
  display: flex;
  min-height: 0;
  min-width: 0;
  flex-direction: column;
  gap: 0.75rem;
  overflow-y: auto;
  border-right: 1px solid rgb(226 232 240);
  background: rgb(248 250 252);
  padding: 0.625rem;
}

.composer-panel-label {
  margin: 0 0 0.375rem;
  padding: 0 0.375rem;
  color: rgb(100 116 139);
  font-size: 0.6875rem;
  font-weight: 700;
  line-height: 1rem;
}

.composer-model-option,
.composer-more-action,
.composer-toolbar-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: 0.375rem;
  transition: background-color 150ms ease, color 150ms ease, border-color 150ms ease;
}

.composer-model-results {
  display: flex;
  min-height: 0;
  min-width: 0;
  flex-direction: column;
  overflow: hidden;
  padding: 0.625rem;
}

.composer-model-search {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.375rem;
  color: rgb(148 163 184);
  padding: 0 0.625rem;
}

.composer-model-search:focus-within {
  border-color: rgb(125 211 252);
  box-shadow: 0 0 0 2px rgb(186 230 253 / 0.72);
}

.composer-model-search input {
  min-width: 0;
  flex: 1;
  border: 0;
  background: transparent;
  padding: 0.5rem 0;
  color: rgb(30 41 59);
  font-size: 0.75rem;
  line-height: 1.25rem;
  outline: 0;
}

.composer-model-list {
  display: grid;
  flex: 1 1 auto;
  min-height: 0;
  align-content: start;
  gap: 0.125rem;
  overscroll-behavior: contain;
  overflow-y: auto;
  padding-top: 0.5rem;
}

.composer-model-option {
  min-height: 2.25rem;
  padding: 0.5rem 0.625rem;
  color: rgb(51 65 85);
  font-size: 0.75rem;
  font-weight: 600;
}

.composer-model-option:hover,
.composer-model-option.is-selected {
  background: rgb(240 249 255);
  color: rgb(3 105 161);
}

.composer-model-empty {
  margin: auto 0;
  padding: 1rem;
  color: rgb(100 116 139);
  font-size: 0.75rem;
  text-align: center;
}

.composer-image-panel {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.5rem;
  padding: 0.625rem;
}

.composer-image-panel > * {
  min-width: 0;
  width: 100%;
}

.composer-video-panel {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.video-option-field {
  display: grid;
  min-width: 0;
  gap: 0.375rem;
}

.video-option-field > span {
  overflow: hidden;
  color: rgb(100 116 139);
  font-size: 0.6875rem;
  font-weight: 700;
  line-height: 1rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dark .video-option-field > span {
  color: rgb(148 163 184);
}

.composer-more-action {
  min-height: 2.25rem;
  padding: 0.5rem 0.625rem;
  color: rgb(71 85 105);
  font-size: 0.75rem;
  font-weight: 600;
}

.composer-more-action:hover:not(:disabled) {
  background: rgb(240 249 255);
  color: rgb(2 132 199);
}

.composer-more-action:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.composer-toolbar {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.125rem;
  overflow-x: auto;
  padding: 0.625rem 0.125rem 0.125rem;
  scrollbar-width: none;
}

.composer-toolbar::-webkit-scrollbar {
  display: none;
}

.composer-toolbar-button {
  min-height: 2.125rem;
  flex: 0 0 auto;
  padding: 0.375rem 0.625rem;
  color: rgb(71 85 105);
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}

.composer-toolbar-button:hover,
.composer-toolbar-button.is-active {
  background: rgb(240 249 255);
  color: rgb(2 132 199);
}

.composer-toolbar-icon-button {
  width: 2.125rem;
  justify-content: center;
  padding: 0;
}

.composer-toolbar-model-button {
  max-width: min(22rem, 52vw);
}

.composer-toolbar-divider {
  height: 1.25rem;
  width: 1px;
  flex: 0 0 1px;
  background: rgb(226 232 240);
  margin: 0 0.25rem;
}

.dark .composer-control-panel {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42);
  box-shadow: 0 14px 30px -24px rgb(0 0 0 / 0.66);
}

.dark .composer-model-key {
  border-color: rgb(51 65 85);
  background: rgb(2 6 23 / 0.48);
}

.dark .composer-panel-label,
.dark .composer-model-empty {
  color: rgb(148 163 184);
}

.dark .composer-model-option,
.dark .composer-more-action,
.dark .composer-toolbar-button {
  color: rgb(203 213 225);
}

.dark .composer-model-option:hover,
.dark .composer-model-option.is-selected,
.dark .composer-more-action:hover:not(:disabled),
.dark .composer-toolbar-button:hover,
.dark .composer-toolbar-button.is-active {
  background: rgb(12 74 110 / 0.32);
  color: rgb(125 211 252);
}

.dark .composer-model-search {
  border-color: rgb(51 65 85);
}

.dark .composer-model-search:focus-within {
  border-color: rgb(3 105 161);
  box-shadow: 0 0 0 2px rgb(7 89 133 / 0.5);
}

.dark .composer-model-search input {
  color: rgb(241 245 249);
}

.dark .composer-toolbar-divider {
  background: rgb(51 65 85);
}

.composer-config-panel {
  display: grid;
  gap: 0.5rem;
}

.composer-runtime-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
}

.composer-runtime-grid {
  display: grid;
  min-width: 0;
  flex: 1;
  grid-template-columns: minmax(10rem, 0.75fr) minmax(16rem, 1.75fr);
  gap: 0.5rem;
}

.composer-runtime-grid > *,
.composer-image-grid > * {
  min-width: 0;
  width: 100%;
}

.composer-config-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.25rem;
}

.composer-config-icon-button {
  display: inline-flex;
  height: 2.25rem;
  width: 2.25rem;
  flex: 0 0 2.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  color: rgb(100 116 139);
  transition: background-color 150ms ease, color 150ms ease;
}

.composer-config-icon-button:hover {
  background: rgb(248 250 252);
  color: rgb(15 23 42);
}

.composer-config-icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.dark .composer-config-icon-button {
  color: rgb(148 163 184);
}

.dark .composer-config-icon-button:hover {
  background: rgb(30 41 59);
  color: rgb(248 250 252);
}

.composer-image-grid {
  display: grid;
  grid-template-columns:
    minmax(8.5rem, 0.82fr)
    minmax(11.5rem, 1.05fr)
    minmax(8rem, 0.72fr)
    minmax(11rem, 1fr)
    minmax(10.5rem, 0.95fr);
  gap: 0.5rem;
  align-items: stretch;
  border-top: 1px solid rgb(226 232 240);
  padding-top: 0.5rem;
}

.dark .composer-image-grid {
  border-top-color: rgb(51 65 85);
}

.image-option-button {
  display: inline-flex;
  min-height: 2.25rem;
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

.image-option-button span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
  margin-left: auto;
  flex-shrink: 0;
  font-weight: 700;
  color: rgb(37 99 235);
}

.image-count-control,
.image-format-control {
  display: inline-flex;
  min-height: 2.25rem;
  align-items: center;
  gap: 0.375rem;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  background: rgb(255 255 255);
  padding: 0.25rem 0.25rem 0.25rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(71 85 105);
}

.image-count-control > span,
.image-format-control > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-count-segments,
.image-format-segments {
  display: inline-grid;
  flex: 0 0 auto;
  margin-left: auto;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.375rem;
}

.image-count-segments {
  grid-template-columns: repeat(4, 1.75rem);
}

.image-format-segments {
  grid-template-columns: repeat(3, minmax(2.75rem, 1fr));
}

.image-count-segments button,
.image-format-segments button {
  height: 1.75rem;
  border-right: 1px solid rgb(226 232 240);
  color: rgb(100 116 139);
  transition: background-color 150ms ease, color 150ms ease;
}

.image-count-segments button:last-child,
.image-format-segments button:last-child {
  border-right: 0;
}

.image-count-segments button:hover,
.image-format-segments button:hover {
  background: rgb(240 249 255);
  color: rgb(2 132 199);
}

.image-count-segments button.is-active,
.image-format-segments button.is-active {
  background: rgb(14 165 233);
  color: rgb(255 255 255);
}

.dark .image-count-control,
.dark .image-format-control {
  border-color: rgb(55 65 81);
  background: rgb(15 23 42);
  color: rgb(203 213 225);
}

.dark .image-count-segments,
.dark .image-format-segments {
  border-color: rgb(55 65 81);
}

.dark .image-count-segments button,
.dark .image-format-segments button {
  border-color: rgb(55 65 81);
  color: rgb(203 213 225);
}

.dark .image-count-segments button:hover,
.dark .image-format-segments button:hover {
  background: rgb(12 74 110 / 0.32);
  color: rgb(125 211 252);
}

.dark .image-count-segments button.is-active,
.dark .image-format-segments button.is-active {
  background: rgb(14 165 233);
  color: rgb(255 255 255);
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

.image-workspace-switch {
  display: inline-flex;
  min-height: 2.25rem;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  background: rgb(255 255 255);
}

.image-workspace-switch button {
  display: inline-flex;
  min-width: 2.25rem;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  padding: 0.375rem 0.625rem;
  color: rgb(100 116 139);
  font-size: 0.75rem;
  font-weight: 600;
  transition: background-color 150ms ease, color 150ms ease;
}

.image-workspace-switch button + button {
  border-left: 1px solid rgb(226 232 240);
}

.image-workspace-switch button:hover {
  background: rgb(240 249 255);
  color: rgb(2 132 199);
}

.image-workspace-switch button.is-active {
  background: rgb(14 165 233);
  color: rgb(255 255 255);
}

.dark .image-workspace-switch {
  border-color: rgb(55 65 81);
  background: rgb(15 23 42);
}

.dark .image-workspace-switch button {
  color: rgb(203 213 225);
}

.dark .image-workspace-switch button + button {
  border-color: rgb(55 65 81);
}

.dark .image-workspace-switch button:hover {
  background: rgb(12 74 110 / 0.32);
  color: rgb(125 211 252);
}

.image-board-root,
.video-board-root {
  position: relative;
  min-height: 50vh;
}

.image-board-root {
  user-select: none;
}

.image-board-grid,
.video-board-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
  padding-bottom: 1.25rem;
}

.image-board-selection-toolbar {
  position: sticky;
  top: 0.5rem;
  z-index: 20;
  display: flex;
  min-height: 3rem;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-bottom: 0.875rem;
  border: 1px solid rgb(203 213 225 / 0.9);
  border-radius: 0.5rem;
  background: rgb(255 255 255 / 0.96);
  padding: 0.5rem 0.75rem;
  color: rgb(51 65 85);
  font-size: 0.8125rem;
  font-weight: 600;
  box-shadow: 0 8px 24px -16px rgb(15 23 42 / 0.5);
  backdrop-filter: blur(10px);
}

.image-board-selection-toolbar > div {
  margin-left: auto;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.image-board-selection-toolbar button {
  display: inline-flex;
  min-height: 2rem;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  border-radius: 0.375rem;
  padding: 0.375rem 0.625rem;
  color: rgb(71 85 105);
  transition: background-color 150ms ease, color 150ms ease;
}

.image-board-selection-toolbar button:hover {
  background: rgb(241 245 249);
  color: rgb(15 23 42);
}

.image-board-selection-toolbar button:disabled {
  cursor: wait;
  opacity: 0.5;
}

.image-board-selection-toolbar button.is-danger {
  background: rgb(254 242 242);
  color: rgb(220 38 38);
}

.image-board-selection-toolbar button.is-danger:hover {
  background: rgb(254 226 226);
}

.image-board-card,
.video-board-card {
  position: relative;
  aspect-ratio: 4 / 3;
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  background: rgb(15 23 42);
  cursor: pointer;
  outline: none;
  transition: box-shadow 180ms ease, border-color 180ms ease, transform 180ms ease;
}

.image-board-card:hover,
.image-board-card:focus-visible,
.video-board-card:hover,
.video-board-card:focus-visible {
  border-color: rgb(148 163 184);
  box-shadow: 0 12px 24px -16px rgb(15 23 42 / 0.55);
  transform: translateY(-1px);
}

.image-board-card.is-error,
.video-board-card.is-error {
  border-color: rgb(254 202 202);
}

.image-board-card.is-selected {
  border-color: rgb(14 165 233);
  box-shadow: 0 0 0 2px rgb(14 165 233 / 0.55);
}

.image-board-card-media {
  position: relative;
  height: 100%;
  width: 100%;
  overflow: hidden;
  background: rgb(248 250 252);
}

.video-board-card-media {
  display: block;
  height: 100%;
  width: 100%;
  object-fit: cover;
  pointer-events: none;
}

.video-board-card-placeholder {
  display: flex;
  height: 100%;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  background: rgb(248 250 252);
  padding: 1.5rem;
  color: rgb(100 116 139);
  text-align: center;
  font-size: 0.75rem;
  line-height: 1.25rem;
}

.video-download-progress {
  display: flex;
  width: min(18rem, 100%);
  min-height: 1rem;
  align-items: center;
  gap: 0.5rem;
}

.video-download-progress-track {
  position: relative;
  height: 0.375rem;
  min-width: 0;
  flex: 1;
  overflow: hidden;
  border-radius: 9999px;
  background: rgb(148 163 184 / 0.3);
}

.video-download-progress-fill {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: rgb(14 165 233);
  transition: width 180ms ease-out;
}

.video-download-progress-fill.is-indeterminate {
  position: absolute;
  left: 0;
  top: 0;
  width: 42%;
  animation: video-download-indeterminate 1.15s ease-in-out infinite;
}

.video-download-progress-label {
  width: 2.5rem;
  flex: none;
  color: currentColor;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  text-align: right;
}

@keyframes video-download-indeterminate {
  from {
    transform: translateX(-110%);
  }
  to {
    transform: translateX(245%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .video-download-progress-fill.is-indeterminate {
    left: 29%;
    animation: none;
  }
}

.image-board-result-count {
  position: absolute;
  left: 0.5rem;
  top: 0.5rem;
  z-index: 5;
  border-radius: 0.25rem;
  background: rgb(15 23 42 / 0.68);
  padding: 0.125rem 0.375rem;
  color: white;
  font-size: 0.625rem;
  font-weight: 700;
  backdrop-filter: blur(6px);
}

.image-board-card-selected {
  position: absolute;
  right: 0.5rem;
  top: 0.5rem;
  z-index: 10;
  display: inline-flex;
  height: 1.25rem;
  width: 1.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: rgb(59 130 246);
  color: rgb(255 255 255);
  box-shadow: 0 1px 3px rgb(15 23 42 / 0.24);
}

.image-board-selection-box {
  position: fixed;
  z-index: 30;
  pointer-events: none;
  border: 1px solid rgb(59 130 246 / 0.5);
  background: rgb(59 130 246 / 0.2);
}

.image-board-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  padding-bottom: 2.5rem;
}

.image-board-pagination button {
  display: inline-flex;
  height: 2rem;
  min-width: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  color: rgb(100 116 139);
  font-size: 0.75rem;
  font-weight: 600;
  transition: background-color 150ms ease, color 150ms ease;
}

.image-board-pagination button:hover:not(:disabled) {
  background: rgb(240 249 255);
  color: rgb(2 132 199);
}

.image-board-pagination button.is-active {
  background: rgb(14 165 233);
  color: rgb(255 255 255);
}

.image-board-pagination button:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}

.image-board-pagination span {
  display: inline-flex;
  height: 2rem;
  min-width: 1rem;
  align-items: center;
  justify-content: center;
  color: rgb(148 163 184);
  font-size: 0.75rem;
}

.image-board-card-overlay,
.video-board-card-overlay {
  position: absolute;
  inset: auto 0 0;
  z-index: 6;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.75rem;
  background: linear-gradient(to top, rgb(2 6 23 / 0.94), rgb(2 6 23 / 0.58) 58%, transparent);
  padding: 4.5rem 0.625rem 0.625rem;
  opacity: 0;
  pointer-events: none;
  transform: translateY(0.375rem);
  transition: opacity 180ms ease, transform 180ms ease;
}

.image-board-card:hover .image-board-card-overlay,
.image-board-card:focus-within .image-board-card-overlay,
.image-board-card.is-selected .image-board-card-overlay,
.video-board-card:hover .video-board-card-overlay,
.video-board-card:focus-within .video-board-card-overlay {
  opacity: 1;
  transform: translateY(0);
}

.image-board-card-meta,
.video-board-card-meta {
  display: grid;
  min-width: 0;
  gap: 0.125rem;
  color: rgb(255 255 255 / 0.78);
  font-size: 0.6875rem;
  line-height: 1rem;
}

.image-board-card-meta span,
.video-board-card-meta span {
  max-width: 10rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-board-card-meta span:first-child,
.video-board-card-meta span:first-child {
  color: white;
  font-weight: 700;
}

.image-board-card-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 0.25rem;
  pointer-events: auto;
}

.image-board-card-action {
  display: inline-flex;
  height: 2.25rem;
  width: 2.25rem;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(255 255 255 / 0.16);
  border-radius: 0.375rem;
  background: rgb(15 23 42 / 0.68);
  color: white;
  backdrop-filter: blur(8px);
  transition: background-color 150ms ease, color 150ms ease;
}

.image-board-card-action:hover {
  background: rgb(255 255 255 / 0.2);
}

.image-board-card-action.is-danger:hover {
  background: rgb(220 38 38 / 0.85);
}

.dark .image-board-selection-toolbar {
  border-color: rgb(255 255 255 / 0.1);
  background: rgb(17 24 39 / 0.94);
  color: rgb(226 232 240);
}

.dark .image-board-selection-toolbar button {
  color: rgb(203 213 225);
}

.dark .image-board-selection-toolbar button:hover {
  background: rgb(255 255 255 / 0.08);
  color: white;
}

.dark .image-board-card,
.dark .video-board-card {
  border-color: rgb(255 255 255 / 0.1);
}

.dark .image-board-pagination button {
  color: rgb(203 213 225);
}

.dark .image-board-pagination button:hover:not(:disabled) {
  background: rgb(12 74 110 / 0.32);
  color: rgb(125 211 252);
}

.dark .image-board-pagination span {
  color: rgb(148 163 184);
}

.dark .image-board-card-media {
  background: rgb(0 0 0 / 0.2);
}

.dark .video-board-card-placeholder {
  background: rgb(3 7 18);
  color: rgb(148 163 184);
}

.image-board-detail-images {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.image-board-detail-image {
  position: relative;
  aspect-ratio: 1;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  background: rgb(248 250 252);
}

.image-board-detail-parameters {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem 1rem;
}

.image-board-detail-parameters div {
  min-width: 0;
  border-bottom: 1px solid rgb(241 245 249);
  padding-bottom: 0.5rem;
}

.image-board-detail-parameters dt {
  color: rgb(100 116 139);
  font-size: 0.6875rem;
  line-height: 1rem;
}

.image-board-detail-parameters dd {
  margin-top: 0.125rem;
  overflow: hidden;
  color: rgb(30 41 59);
  font-size: 0.8125rem;
  font-weight: 600;
  line-height: 1.25rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dark .image-board-detail-image {
  border-color: rgb(55 65 81);
  background: rgb(2 6 23);
}

.dark .image-board-detail-parameters div {
  border-color: rgb(30 41 59);
}

.dark .image-board-detail-parameters dt {
  color: rgb(148 163 184);
}

.dark .image-board-detail-parameters dd {
  color: rgb(226 232 240);
}

.playground-image-strip {
  display: flex;
  width: max-content;
  max-width: min(43.5rem, calc(100vw - 7rem));
  gap: 0.5rem;
  overflow-x: auto;
  padding-bottom: 0.25rem;
  overscroll-behavior-x: contain;
  scrollbar-width: thin;
}

.playground-image-thumbnail {
  aspect-ratio: 1;
  width: 10.5rem;
  flex: 0 0 10.5rem;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  background: rgb(248 250 252);
}

.dark .playground-image-thumbnail {
  border-color: rgb(55 65 81);
  background: rgb(15 23 42);
}

.doodle-canvas-stage {
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(241 245 249);
}

.doodle-sticker-panel {
  background: rgb(248 250 252);
}

.doodle-sticker-button {
  display: inline-flex;
  height: 2.125rem;
  width: 2.125rem;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: 0.375rem;
  background: rgb(255 255 255);
  font-size: 1.125rem;
  line-height: 1;
  transition: background-color 150ms ease, border-color 150ms ease, transform 150ms ease;
}

.doodle-sticker-button:hover:not(:disabled),
.doodle-sticker-button.is-active {
  border-color: rgb(125 211 252);
  background: rgb(240 249 255);
  transform: translateY(-1px);
}

.doodle-sticker-button:disabled {
  cursor: wait;
  opacity: 0.45;
}

.dark .doodle-canvas-stage {
  background: rgb(2 6 23);
}

.dark .doodle-sticker-panel {
  background: rgb(2 6 23 / 0.48);
}

.dark .doodle-sticker-button {
  background: rgb(15 23 42);
}

.dark .doodle-sticker-button:hover:not(:disabled),
.dark .doodle-sticker-button.is-active {
  border-color: rgb(3 105 161);
  background: rgb(12 74 110 / 0.32);
}

.doodle-tool-button,
.doodle-action-button {
  display: inline-flex;
  height: 2.25rem;
  width: 2.25rem;
  align-items: center;
  justify-content: center;
  color: rgb(100 116 139);
  transition: background-color 150ms ease, color 150ms ease;
}

.doodle-tool-button + .doodle-tool-button {
  border-left: 1px solid rgb(226 232 240);
}

.doodle-tool-button:hover,
.doodle-action-button:hover {
  background: rgb(240 249 255);
  color: rgb(2 132 199);
}

.doodle-tool-button.is-active {
  background: rgb(14 165 233);
  color: white;
}

.doodle-action-button {
  border-radius: 0.5rem;
}

.doodle-action-button:disabled {
  cursor: not-allowed;
  opacity: 0.35;
}

.dark .doodle-tool-button,
.dark .doodle-action-button {
  color: rgb(203 213 225);
}

.dark .doodle-tool-button + .doodle-tool-button {
  border-color: rgb(55 65 81);
}

.dark .doodle-tool-button:hover,
.dark .doodle-action-button:hover {
  background: rgb(12 74 110 / 0.32);
  color: rgb(125 211 252);
}

.dark .doodle-tool-button.is-active {
  background: rgb(14 165 233);
  color: white;
}

@media (max-width: 767px) {
  .playground-mobile-header {
    position: fixed;
    top: 4rem;
    left: 0;
    right: 0;
    z-index: 20;
    background: rgb(248 250 252);
    box-shadow: 0 1px 0 rgb(15 23 42 / 0.08), 0 10px 18px -18px rgb(15 23 42 / 0.34);
  }

  .dark .playground-mobile-header {
    background: rgb(3 7 18);
    box-shadow: 0 1px 0 rgb(255 255 255 / 0.08), 0 10px 18px -18px rgb(0 0 0 / 0.72);
  }

  .playground-mobile-scroller {
    padding-top: 6rem;
  }

  .playground-mobile-composer {
    position: fixed;
    padding-bottom: calc(1rem + env(safe-area-inset-bottom));
  }

  .composer-model-picker {
    max-height: min(26rem, calc(100svh - 10rem));
  }
}

@media (max-width: 640px) {
  .playground-image-strip {
    max-width: calc(100vw - 5.5rem);
  }

  .playground-image-thumbnail {
    width: 9rem;
    flex-basis: 9rem;
  }

  .image-board-detail-images {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (min-width: 640px) {
  .image-board-grid,
  .video-board-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1024px) {
  .image-board-grid,
  .video-board-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (hover: none) {
  .image-board-card-overlay,
  .video-board-card-overlay {
    opacity: 1;
    transform: translateY(0);
  }

  .playground-thumbnail-actions {
    align-items: flex-start;
    justify-content: flex-end;
    padding: 0.375rem;
    background: transparent;
    opacity: 1;
  }

  .playground-thumbnail-actions button:last-child {
    display: none;
  }

  .playground-attachment-edit {
    opacity: 1;
    pointer-events: auto;
    background: rgb(15 23 42 / 0.38);
  }
}

.playground-attachment-chip,
.playground-pending-attachment {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 0.25rem;
  border-radius: 9999px;
  border: 1px solid rgb(226 232 240);
  background: rgb(248 250 252);
  padding: 0.25rem 0.5rem;
  font-size: 0.75rem;
  color: rgb(100 116 139);
}

.playground-pending-attachment {
  position: relative;
  background: rgb(255 255 255);
}

.playground-attachment-chip-image,
.playground-pending-attachment-image {
  height: 3.5rem;
  width: 3.5rem;
  overflow: hidden;
  border-radius: 0.5rem;
  padding: 0;
}

.playground-attachment-edit {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(15 23 42 / 0.46);
  color: rgb(255 255 255);
  opacity: 0;
  pointer-events: none;
  transition: opacity 150ms ease, background-color 150ms ease;
}

.playground-pending-attachment-image:hover .playground-attachment-edit,
.playground-pending-attachment-image:focus-within .playground-attachment-edit {
  opacity: 1;
  pointer-events: auto;
}

.playground-attachment-edit:hover {
  background: rgb(14 116 144 / 0.66);
}

.playground-attachment-chip-image {
  cursor: zoom-in;
}

.playground-attachment-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: rgb(148 163 184);
  transition: color 150ms ease;
}

.playground-pending-attachment-image .playground-attachment-remove {
  position: absolute;
  right: 0.125rem;
  top: 0.125rem;
  z-index: 20;
  height: 1.125rem;
  width: 1.125rem;
  border-radius: 9999px;
  background: rgb(15 23 42 / 0.72);
  color: white;
}

.playground-attachment-remove:hover {
  color: rgb(239 68 68);
}

.dark .playground-attachment-chip,
.dark .playground-pending-attachment {
  border-color: rgb(55 65 81);
  background: rgb(30 41 59);
  color: rgb(203 213 225);
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
  min-height: 2.25rem;
  border-radius: 0.5rem;
  padding: 0.375rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 600;
}

.playground-compact-select :deep(.select-trigger) {
  height: 2.25rem;
}

.playground-key-select :deep(.select-trigger) {
  height: auto;
  align-items: flex-start;
}

.playground-key-select :deep(.select-value) {
  overflow: visible;
  white-space: normal;
  text-overflow: clip;
}

.playground-key-select-value {
  overflow-wrap: anywhere;
}

@container playground-composer (max-width: 860px) {
  .composer-image-panel {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .composer-image-panel > :nth-child(n + 4) {
    grid-column: span 1;
  }

  .composer-image-grid {
    grid-template-columns: repeat(6, minmax(0, 1fr));
  }

  .composer-image-grid > :nth-child(-n + 3) {
    grid-column: span 2;
  }

  .composer-image-grid > :nth-child(n + 4) {
    grid-column: span 3;
  }
}

@container playground-composer (max-width: 640px) {
  .composer-model-picker {
    grid-template-columns: 9.5rem minmax(0, 1fr);
  }

  .playground-key-select :deep(.select-trigger) {
    height: 2.5rem;
    align-items: center;
  }

  .playground-key-select :deep(.select-value),
  .playground-key-select-value {
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .playground-key-select-value {
    overflow-wrap: normal;
  }

  .playground-key-select-label {
    min-width: 0;
    flex: 1 1 0%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .composer-image-panel {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .composer-image-panel > :nth-child(n) {
    grid-column: auto;
  }

  .composer-toolbar-button {
    min-height: 2.5rem;
  }

  .composer-toolbar-icon-button {
    width: 2.5rem;
  }

  .composer-runtime-row {
    flex-direction: column;
    align-items: stretch;
  }

  .composer-runtime-grid {
    width: 100%;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .composer-config-actions {
    order: -1;
    align-self: flex-end;
  }

  .composer-config-icon-button,
  .image-option-button,
  .image-count-control,
  .image-format-control {
    min-height: 2.5rem;
  }

  .composer-config-icon-button {
    height: 2.5rem;
    width: 2.5rem;
    flex-basis: 2.5rem;
  }

  .composer-image-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .composer-image-grid > :nth-child(1),
  .composer-image-grid > :nth-child(3) {
    grid-column: span 1;
  }

  .composer-image-grid > :nth-child(2),
  .composer-image-grid > :nth-child(4),
  .composer-image-grid > :nth-child(5) {
    grid-column: 1 / -1;
  }

  .playground-compact-select :deep(.select-trigger),
  .image-option-select :deep(.select-trigger) {
    min-height: 2.5rem;
  }

  .playground-compact-select :deep(.select-trigger) {
    height: 2.5rem;
  }

  .image-count-segments button,
  .image-format-segments button {
    height: 2rem;
  }
}

@container playground-composer (max-width: 420px) {
  .composer-model-picker {
    grid-template-columns: 9rem minmax(0, 1fr);
  }

  .composer-model-option {
    padding-left: 0.5rem;
    padding-right: 0.5rem;
  }

  .composer-runtime-grid,
  .composer-image-grid,
  .composer-video-panel {
    grid-template-columns: minmax(0, 1fr);
  }

  .composer-model-select,
  .composer-image-grid > :nth-child(n) {
    grid-column: auto;
  }
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
