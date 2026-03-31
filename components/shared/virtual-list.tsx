"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

export interface VirtualListProps<T>
  extends Omit<React.HTMLAttributes<HTMLDivElement>, "children"> {
  items: T[];
  height: number;
  itemHeight: number;
  overscan?: number;
  itemKey?: (item: T, index: number) => React.Key;
  renderItem: (item: T, index: number) => React.ReactNode;
  emptyState?: React.ReactNode;
}

export function VirtualList<T>({
  items,
  height,
  itemHeight,
  overscan = 2,
  itemKey,
  renderItem,
  emptyState = null,
  className,
  style,
  ...props
}: VirtualListProps<T>) {
  const [scrollTop, setScrollTop] = React.useState(0);
  const totalHeight = items.length * itemHeight;
  const startIndex = Math.max(0, Math.floor(scrollTop / itemHeight) - overscan);
  const endIndex = Math.min(
    items.length,
    Math.ceil((scrollTop + height) / itemHeight) + overscan,
  );
  const visibleItems = React.useMemo(
    () =>
      items.slice(startIndex, endIndex).map((item, offset) => ({
        item,
        index: startIndex + offset,
      })),
    [endIndex, items, startIndex],
  );

  return (
    <div
      className={cn("overflow-y-auto", className)}
      style={{ height, ...style }}
      onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
      {...props}
    >
      {items.length === 0 ? (
        emptyState
      ) : (
        <div style={{ height: totalHeight, position: "relative" }}>
          {visibleItems.map(({ item, index }) => (
            <div
              key={itemKey ? itemKey(item, index) : index}
              style={{
                position: "absolute",
                top: index * itemHeight,
                left: 0,
                right: 0,
                height: itemHeight,
              }}
            >
              {renderItem(item, index)}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
