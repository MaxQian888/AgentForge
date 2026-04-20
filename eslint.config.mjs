import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

// Banned raw Tailwind spacing tokens on dashboard page JSX roots. These are the
// tokens that must be replaced with design-token CSS variables
// (var(--space-section-gap), var(--space-grid-gap), var(--space-page-inline),
// var(--space-card-padding)). Errors per the `refine-ui-design-cohesion`
// change phase 7 lock-in.
const BANNED_PAGE_ROOT_CLASSES = [
  "gap-6",
  "p-6",
  "space-y-6",
  "px-6",
  "py-6",
];

const bannedClassPattern = BANNED_PAGE_ROOT_CLASSES.map((cls) =>
  cls.replace(/-/g, "\\-"),
).join("|");

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "coverage/**",
    "src-tauri/target/**",
    "next-env.d.ts",
  ]),
  {
    // Dashboard-page spacing contract per ui-design-consistency spec.
    // JSX roots under app/(dashboard)/**/page.tsx must use design-token
    // spacing (var(--space-*)) instead of raw Tailwind gap-*/p-*/space-y-*
    // utilities. See openspec/specs/ui-design-consistency/spec.md and
    // docs/guides/frontend-components.md for the full contract.
    files: ["app/(dashboard)/**/page.tsx"],
    rules: {
      "no-restricted-syntax": [
        "error",
        {
          selector: `JSXAttribute[name.name="className"] > Literal[value=/\\b(${bannedClassPattern})\\b/]`,
          message:
            "Use design-token spacing (e.g., gap-[var(--space-section-gap)], p-[var(--space-page-inline)], gap-[var(--space-grid-gap)]) on dashboard pages instead of raw Tailwind gap-6/p-6/space-y-6/px-6/py-6. See docs/guides/frontend-components.md for the full contract.",
        },
        {
          selector: `JSXAttribute[name.name="className"] > JSXExpressionContainer > TemplateLiteral > TemplateElement[value.raw=/\\b(${bannedClassPattern})\\b/]`,
          message:
            "Use design-token spacing (e.g., gap-[var(--space-section-gap)], p-[var(--space-page-inline)]) on dashboard pages instead of raw Tailwind gap-6/p-6/space-y-6/px-6/py-6. See docs/guides/frontend-components.md.",
        },
      ],
    },
  },
]);

export default eslintConfig;
