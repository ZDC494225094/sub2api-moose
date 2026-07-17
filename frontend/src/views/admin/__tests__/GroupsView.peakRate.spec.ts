import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

import { describe, expect, it } from "vitest";

const currentDir = dirname(fileURLToPath(import.meta.url));
const groupsViewSource = readFileSync(
  resolve(currentDir, "../GroupsView.vue"),
  "utf8",
);

describe("admin group peak rate controls", () => {
  it("shows the existing peak rate controls for standard and subscription groups", () => {
    expect(groupsViewSource).toContain('v-model="createForm.peak_rate_enabled"');
    expect(groupsViewSource).toContain('v-model="editForm.peak_rate_enabled"');
    expect(
      groupsViewSource.match(
        /<!-- 高峰时段倍率配置 -->\s*<div class="border-t pt-4">/g,
      ),
    ).toHaveLength(2);
    expect(groupsViewSource).not.toContain(
      "() => editForm.subscription_type,",
    );
    expect(groupsViewSource).toContain(
      'extractApiErrorMessage(error, t("admin.groups.failedToUpdate"))',
    );
    expect(groupsViewSource).toContain(
      'extractApiErrorMessage(error, t("admin.groups.failedToCreate"))',
    );
  });
});
