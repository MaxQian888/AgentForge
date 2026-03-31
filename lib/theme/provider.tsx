"use client";

import {
  createContext,
  memo,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type DetailedHTMLProps,
  type Dispatch,
  type PropsWithChildren,
  type ScriptHTMLAttributes,
  type SetStateAction,
} from "react";

interface ValueObject {
  [themeName: string]: string;
}

type DataAttribute = `data-${string}`;

interface ScriptProps
  extends DetailedHTMLProps<
    ScriptHTMLAttributes<HTMLScriptElement>,
    HTMLScriptElement
  > {
  [dataAttribute: DataAttribute]: unknown;
}

export interface UseThemeProps {
  forcedTheme?: string | undefined;
  resolvedTheme?: string | undefined;
  setTheme: Dispatch<SetStateAction<string>>;
  systemTheme?: "dark" | "light" | undefined;
  theme?: string | undefined;
  themes: string[];
}

export type Attribute = DataAttribute | "class";

export interface ThemeProviderProps extends PropsWithChildren {
  attribute?: Attribute | Attribute[] | undefined;
  defaultTheme?: string | undefined;
  disableTransitionOnChange?: boolean | undefined;
  enableColorScheme?: boolean | undefined;
  enableSystem?: boolean | undefined;
  forcedTheme?: string | undefined;
  nonce?: string;
  scriptProps?: ScriptProps;
  storageKey?: string | undefined;
  themes?: string[] | undefined;
  value?: ValueObject | undefined;
}

const MEDIA = "(prefers-color-scheme: dark)";
const COLOR_SCHEMES = ["light", "dark"] as const;
const DEFAULT_THEMES = ["light", "dark"];
const isServer = typeof window === "undefined";

const ThemeContext = createContext<UseThemeProps | undefined>(undefined);
const defaultContext: UseThemeProps = {
  setTheme: () => {},
  themes: [],
};

function applyThemeScript(
  attribute: Attribute | Attribute[],
  storageKey: string,
  defaultTheme: string,
  forcedTheme: string | undefined,
  themes: string[],
  value: ValueObject | undefined,
  enableSystem: boolean,
  enableColorScheme: boolean,
) {
  const root = document.documentElement;
  const colorSchemes = ["light", "dark"];

  const setColorScheme = (theme: string) => {
    if (enableColorScheme && colorSchemes.includes(theme)) {
      root.style.colorScheme = theme;
    }
  };

  const applyAttribute = (theme: string) => {
    const attributes = Array.isArray(attribute) ? attribute : [attribute];

    attributes.forEach((entry) => {
      const isClass = entry === "class";
      const values = isClass && value ? themes.map((name) => value[name] ?? name) : themes;

      if (isClass) {
        root.classList.remove(...values);
        root.classList.add(value?.[theme] ?? theme);
        return;
      }

      root.setAttribute(entry, value?.[theme] ?? theme);
    });

    setColorScheme(theme);
  };

  const resolveSystemTheme = () =>
    window.matchMedia(MEDIA).matches ? "dark" : "light";

  if (forcedTheme) {
    applyAttribute(forcedTheme);
    return;
  }

  try {
    const storedTheme = localStorage.getItem(storageKey) || defaultTheme;
    const resolvedTheme =
      enableSystem && storedTheme === "system"
        ? resolveSystemTheme()
        : storedTheme;

    applyAttribute(resolvedTheme);
  } catch {}
}

function getTheme(storageKey: string, fallback?: string) {
  if (isServer) {
    return undefined;
  }

  try {
    return localStorage.getItem(storageKey) ?? fallback;
  } catch {
    return fallback;
  }
}

function getSystemTheme(mediaQuery?: MediaQueryList) {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return "light";
  }

  const source = mediaQuery ?? window.matchMedia(MEDIA);
  return source.matches ? "dark" : "light";
}

function isColorScheme(theme: string): theme is (typeof COLOR_SCHEMES)[number] {
  return (COLOR_SCHEMES as readonly string[]).includes(theme);
}

function disableAnimation(nonce?: string) {
  const style = document.createElement("style");

  if (nonce) {
    style.setAttribute("nonce", nonce);
  }

  style.appendChild(
    document.createTextNode(
      "*,*::before,*::after{-webkit-transition:none!important;-moz-transition:none!important;-o-transition:none!important;-ms-transition:none!important;transition:none!important}",
    ),
  );
  document.head.appendChild(style);

  return () => {
    window.getComputedStyle(document.body);
    setTimeout(() => {
      document.head.removeChild(style);
    }, 1);
  };
}

export function useTheme() {
  return useContext(ThemeContext) ?? defaultContext;
}

export function ThemeProvider(props: ThemeProviderProps) {
  const context = useContext(ThemeContext);

  if (context) {
    return <>{props.children}</>;
  }

  return <ManagedThemeProvider {...props} />;
}

function ManagedThemeProvider({
  forcedTheme,
  disableTransitionOnChange = false,
  enableSystem = true,
  enableColorScheme = true,
  storageKey = "theme",
  themes = DEFAULT_THEMES,
  defaultTheme = enableSystem ? "system" : "light",
  attribute = "data-theme",
  value,
  children,
  nonce,
  scriptProps,
}: ThemeProviderProps) {
  const [theme, setThemeState] = useState(() => getTheme(storageKey, defaultTheme));
  const [resolvedTheme, setResolvedTheme] = useState<"light" | "dark" | undefined>(() =>
    enableSystem ? getSystemTheme() : undefined,
  );
  const attributeValues = value ? Object.values(value) : themes;

  const applyTheme = useCallback(
    (nextTheme: string) => {
      if (!nextTheme) {
        return;
      }

      let resolved = nextTheme;
      if (nextTheme === "system" && enableSystem) {
        resolved = getSystemTheme();
      }

      const restoreTransitions = disableTransitionOnChange
        ? disableAnimation(nonce)
        : null;
      const root = document.documentElement;

      const applyAttribute = (entry: Attribute) => {
        if (entry === "class") {
          root.classList.remove(...attributeValues);
          if (resolved) {
            root.classList.add(value?.[resolved] ?? resolved);
          }
          return;
        }

        if (resolved) {
          root.setAttribute(entry, value?.[resolved] ?? resolved);
          return;
        }

        root.removeAttribute(entry);
      };

      if (Array.isArray(attribute)) {
        attribute.forEach(applyAttribute);
      } else {
        applyAttribute(attribute);
      }

      if (enableColorScheme) {
        const fallbackScheme = isColorScheme(defaultTheme)
          ? defaultTheme
          : undefined;
        const colorScheme = isColorScheme(resolved) ? resolved : fallbackScheme;
        root.style.colorScheme = colorScheme ?? "";
      }

      restoreTransitions?.();
    },
    [
      attribute,
      attributeValues,
      defaultTheme,
      disableTransitionOnChange,
      enableColorScheme,
      enableSystem,
      nonce,
      value,
    ],
  );

  const setTheme = useCallback<Dispatch<SetStateAction<string>>>(
    (valueOrUpdater) => {
      const nextTheme =
        typeof valueOrUpdater === "function"
          ? valueOrUpdater(theme ?? defaultTheme)
          : valueOrUpdater;

      setThemeState(nextTheme);
      try {
        localStorage.setItem(storageKey, nextTheme);
      } catch {}
    },
    [defaultTheme, storageKey, theme],
  );

  const handleMediaQuery = useCallback(
    (source?: { matches: boolean }) => {
      const systemTheme = source ? (source.matches ? "dark" : "light") : getSystemTheme();
      setResolvedTheme(systemTheme);

      if (theme === "system" && enableSystem && !forcedTheme) {
        applyTheme("system");
      }
    },
    [applyTheme, enableSystem, forcedTheme, theme],
  );

  useEffect(() => {
    if (typeof window.matchMedia !== "function") {
      return;
    }

    const mediaQuery = window.matchMedia(MEDIA);
    const handleChange = () => handleMediaQuery(mediaQuery);

    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", handleChange);
      return () => {
        mediaQuery.removeEventListener("change", handleChange);
      };
    }

    mediaQuery.addListener(handleMediaQuery);
    return () => {
      mediaQuery.removeListener(handleMediaQuery);
    };
  }, [handleMediaQuery]);

  useEffect(() => {
    const handleStorage = (event: StorageEvent) => {
      if (event.key !== storageKey) {
        return;
      }

      if (event.newValue) {
        setThemeState(event.newValue);
        return;
      }

      setTheme(defaultTheme);
    };

    window.addEventListener("storage", handleStorage);
    return () => {
      window.removeEventListener("storage", handleStorage);
    };
  }, [defaultTheme, setTheme, storageKey]);

  useEffect(() => {
    applyTheme(forcedTheme ?? theme ?? defaultTheme);
  }, [applyTheme, defaultTheme, forcedTheme, theme]);

  const providerValue = useMemo<UseThemeProps>(
    () => ({
      forcedTheme,
      resolvedTheme: theme === "system" ? resolvedTheme : theme,
      setTheme,
      systemTheme: enableSystem ? resolvedTheme : undefined,
      theme,
      themes: enableSystem ? [...themes, "system"] : themes,
    }),
    [enableSystem, forcedTheme, resolvedTheme, setTheme, theme, themes],
  );

  return (
    <ThemeContext.Provider value={providerValue}>
      <ThemeScript
        attribute={attribute}
        defaultTheme={defaultTheme}
        enableColorScheme={enableColorScheme}
        enableSystem={enableSystem}
        forcedTheme={forcedTheme}
        nonce={nonce}
        scriptProps={scriptProps}
        storageKey={storageKey}
        themes={themes}
        value={value}
      />
      {children}
    </ThemeContext.Provider>
  );
}

const ThemeScript = memo(function ThemeScript({
  forcedTheme,
  storageKey,
  attribute,
  enableSystem,
  enableColorScheme,
  defaultTheme,
  value,
  themes,
  nonce,
  scriptProps,
}: Omit<ThemeProviderProps, "children" | "disableTransitionOnChange">) {
  // React 19 warns when client components render <script>; keep the boot script SSR-only.
  if (typeof window !== "undefined") {
    return null;
  }

  const scriptArgs = JSON.stringify([
    attribute,
    storageKey,
    defaultTheme,
    forcedTheme,
    themes,
    value,
    enableSystem,
    enableColorScheme,
  ]).slice(1, -1);

  return (
    <script
      {...scriptProps}
      suppressHydrationWarning
      nonce={nonce}
      dangerouslySetInnerHTML={{
        __html: `(${applyThemeScript.toString()})(${scriptArgs})`,
      }}
    />
  );
});
