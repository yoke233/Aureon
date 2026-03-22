import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";

const APP_URL = process.env.APP_URL ?? "http://localhost:5173";
const APP_TOKEN = process.env.APP_TOKEN ?? "";

const ensureAuthenticated = async (page: Page) => {
  const loginHeading = page.getByRole("heading", { name: /登录|Login/ });
  if (!(await loginHeading.isVisible().catch(() => false))) {
    return;
  }

  if (!APP_TOKEN) {
    throw new Error("APP_TOKEN is required when the app redirects to the login page.");
  }

  await page.getByLabel("API Token").fill(APP_TOKEN);
  await page.getByRole("button", { name: /登录|Login/ }).click();
};

test("项目创建页可创建项目并绑定本地工作目录", async ({ page }) => {
  const runID = Date.now();
  const projectName = `e2e-project-${runID}`;
  const workingDir = "D:/project/ai-workflow";
  const createProjectUrl = new URL(APP_URL);
  createProjectUrl.pathname = "/projects/new";

  await page.goto(createProjectUrl.toString(), { waitUntil: "networkidle" });
  await ensureAuthenticated(page);
  await expect(page.getByRole("heading", { name: "新建项目" })).toBeVisible({ timeout: 15_000 });

  await page.getByPlaceholder("例如：ai-workflow").fill(projectName);
  await page.getByPlaceholder("描述项目的目标、技术栈和范围...").fill("Playwright e2e project creation regression");
  await page.getByPlaceholder("D:/project/my-repo").fill(workingDir);
  await page.getByRole("button", { name: "创建项目" }).click();

  await expect(page).toHaveURL(/\/projects$/);
  await expect(page.getByRole("heading", { name: "项目" })).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(projectName).first()).toBeVisible({ timeout: 60_000 });

  const screenshotPath = join(".runtime", "playwright", `project-creation-${runID}.png`);
  mkdirSync(dirname(screenshotPath), { recursive: true });
  await page.screenshot({ path: screenshotPath, fullPage: true });
});
