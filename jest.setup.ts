/**
 * Jest setup file
 * This file is executed before each test file
 */

import '@testing-library/jest-dom';
import React from 'react';
import enMessages from "@/messages/en";

type MockNextImageProps = React.ComponentPropsWithoutRef<'img'> & {
  priority?: boolean;
  fill?: boolean;
};

// Mock Next.js Image component
jest.mock('next/image', () => ({
  __esModule: true,
  default: (props: MockNextImageProps) => {
    const normalizedProps = { ...props };
    delete normalizedProps.priority;
    delete normalizedProps.fill;
    return React.createElement('img', normalizedProps);
  },
}));

// Mock Next.js router
jest.mock('next/navigation', () => ({
  useRouter() {
    return {
      push: jest.fn(),
      replace: jest.fn(),
      prefetch: jest.fn(),
      back: jest.fn(),
      pathname: '/',
      query: {},
      asPath: '/',
    };
  },
  usePathname() {
    return '/';
  },
  useSearchParams() {
    return new URLSearchParams();
  },
}));

function resolveTranslationValue(source: unknown, path: string) {
  const segments = path.split(".");
  let current: unknown = source;

  for (const segment of segments) {
    if (!current || typeof current !== "object" || Array.isArray(current)) {
      return undefined;
    }
    current = (current as Record<string, unknown>)[segment];
  }

  return typeof current === "string" ? current : undefined;
}

function interpolateTranslation(
  message: string,
  values?: Record<string, string | number>,
) {
  if (!values) {
    return message;
  }

  return Object.entries(values).reduce((output, [name, value]) => {
    return output.replace(new RegExp(`\\{${name}\\}`, "g"), String(value));
  }, message);
}

jest.mock("next-intl", () => ({
  NextIntlClientProvider: ({
    children,
  }: {
    children: React.ReactNode;
    locale?: string;
    messages?: Record<string, unknown>;
  }) => React.createElement(React.Fragment, null, children),
  useTranslations:
    (namespace?: string) =>
    (key: string, values?: Record<string, string | number>) => {
      const resolvedPath = namespace ? `${namespace}.${key}` : key;
      const message = resolveTranslationValue(enMessages, resolvedPath) ?? resolvedPath;
      return interpolateTranslation(message, values);
    },
}));

class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (typeof global.ResizeObserver === "undefined") {
  global.ResizeObserver = MockResizeObserver as typeof ResizeObserver;
}

if (typeof window !== "undefined" && typeof window.matchMedia === "undefined") {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

if (typeof Element !== "undefined") {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }

  if (!Element.prototype.setPointerCapture) {
    Element.prototype.setPointerCapture = () => {};
  }

  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }

  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }
}

// Suppress console errors in tests (optional)
// global.console = {
//   ...console,
//   error: jest.fn(),
//   warn: jest.fn(),
// };

