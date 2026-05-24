import { useEffect, useState } from 'react';

const MOBILE_BREAKPOINT_PX = 768;

export function useMediaQuery(breakpoint: number = MOBILE_BREAKPOINT_PX) {
  const [isMobile, setIsMobile] = useState<boolean>(() => window.innerWidth <= breakpoint);

  useEffect(() => {
    const onResize = () => setIsMobile(window.innerWidth <= breakpoint);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [breakpoint]);

  return { isMobile };
}
