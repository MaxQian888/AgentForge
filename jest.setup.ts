/**
 * Jest setup file
 * This file is executed before each test file
 */

import '@testing-library/jest-dom';
import React from 'react';

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

class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (typeof global.ResizeObserver === "undefined") {
  global.ResizeObserver = MockResizeObserver as typeof ResizeObserver;
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

