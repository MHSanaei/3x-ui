import { ref, onBeforeUnmount, onMounted } from 'vue';

const MOBILE_BREAKPOINT_PX = 768;

// Vue 3 replacement for the legacy MediaQueryMixin. Returns a reactive
// `isMobile` ref that updates on window resize. Use inside <script setup>:
//
//   const { isMobile } = useMediaQuery();
export function useMediaQuery(breakpoint = MOBILE_BREAKPOINT_PX) {
  const compute = () => window.innerWidth <= breakpoint;
  const isMobile = ref(compute());

  const onResize = () => {
    isMobile.value = compute();
  };

  onMounted(() => {
    window.addEventListener('resize', onResize);
  });

  onBeforeUnmount(() => {
    window.removeEventListener('resize', onResize);
  });

  return { isMobile };
}
