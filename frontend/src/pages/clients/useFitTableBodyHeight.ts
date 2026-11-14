import { useLayoutEffect, useState } from 'react';

const MIN_BODY_HEIGHT = 240;

// Height for the table body so the whole card (toolbar, header, rows, horizontal
// scrollbar, pagination) fits the viewport; long pages then scroll inside the table.
export function useFitTableBodyHeight(container: HTMLElement | null): number | undefined {
  const [height, setHeight] = useState<number>();

  useLayoutEffect(() => {
    if (!container) return;
    const contentArea = container.closest<HTMLElement>('.content-area');
    const card = container.closest<HTMLElement>('.ant-card') ?? container;

    const measure = () => {
      const header = container.querySelector<HTMLElement>('.ant-table-thead');
      const table = container.querySelector<HTMLElement>('.ant-table');
      if (!header || !table) return;
      const bodyTop = header.getBoundingClientRect().bottom + window.scrollY;
      const pagePadding = contentArea ? parseFloat(getComputedStyle(contentArea).paddingBottom) : 0;
      const below =
        card.getBoundingClientRect().bottom - table.getBoundingClientRect().bottom + pagePadding;
      setHeight(Math.max(MIN_BODY_HEIGHT, Math.floor(window.innerHeight - bodyTop - below)));
    };

    measure();
    // The page area keeps a min-height, so it does not resize when a filter-chip row
    // disappears; the cards themselves do.
    const observer = new ResizeObserver(measure);
    const cards = contentArea?.querySelectorAll<HTMLElement>('.ant-card') ?? [card];
    cards.forEach((el) => observer.observe(el));
    window.addEventListener('resize', measure);
    return () => {
      observer.disconnect();
      window.removeEventListener('resize', measure);
    };
  }, [container]);

  return container ? height : undefined;
}
