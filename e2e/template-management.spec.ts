import { test, expect } from "@playwright/test"

test("docs template harness supports create and instantiate flows", async ({ page }) => {
  await page.goto("/playwright/docs-template")

  await page.getByRole("button", { name: "New Template" }).click()
  await page.getByLabel("Template Title").fill("Ops Playbook")
  await page.getByRole("button", { name: "Create Template" }).click()

  await expect(page.getByRole("button", { name: /Ops Playbook/ }).first()).toBeVisible()

  await page.getByRole("button", { name: "Use Template" }).click()
  await page.getByLabel("Document Title").fill("Ops Playbook Draft")
  await page.getByLabel("Destination").selectOption("ops-folder")
  await page.getByRole("button", { name: "Create Document" }).click()

  await expect(page.getByTestId("created-documents")).toContainText("Ops Playbook Draft")
  await expect(page.getByTestId("created-documents")).toContainText("ops-folder")
})

test("workflow template harness supports publish and clone flows", async ({ page }) => {
  await page.goto("/playwright/workflow-template")

  await page.getByRole("button", { name: "Publish Delivery Flow" }).click()
  await expect(page.getByRole("button", { name: /Delivery Flow Template/ }).first()).toBeVisible()

  await page.getByRole("button", { name: "Custom" }).click()
  await page.getByRole("button", { name: "Clone" }).click()
  await expect(page.getByText("Create Workflow Copy: Delivery Flow Template")).toBeVisible()
  await page.getByRole("button", { name: "Create Workflow Copy" }).click()

  await expect(page.getByTestId("workflow-active-tab")).toContainText("workflows")
  await expect(page.getByTestId("workflow-definition-count")).toContainText("2")
})
