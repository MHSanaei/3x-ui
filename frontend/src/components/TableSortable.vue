<script>
// Use defineComponent so we can keep the parent + child components in
// the same file with the provide() <-> inject relationship intact.
import { defineComponent, h, computed, ref, resolveComponent, inject } from 'vue';
import { DragOutlined } from '@ant-design/icons-vue';

const ROW_CLASS = 'sortable-row';

// Sortable a-table — drag-to-reorder rows using Pointer Events.
//
// Why a custom component:
// - Old impl set draggable: true on every row, which broke text selection
//   in cells and let HTML5 start drags from anywhere on the row. This
//   version only initiates drag from an explicit handle, via Pointer
//   Events (one API for mouse + touch + pen).
// - During drag, data-source is reordered live; the source row visually
//   slides into the target slot. The live reorder IS the visual feedback.
// - On commit, emits onsort(sourceIndex, targetIndex) — same signature as
//   before so existing call sites stay unchanged.
// - Keyboard support: ArrowUp/ArrowDown move the focused handle's row by
//   one; Escape cancels an in-flight drag.

export const TableSortableTrigger = defineComponent({
  name: 'TableSortableTrigger',
  props: {
    itemIndex: { type: Number, required: true },
  },
  setup(props) {
    const sortable = inject('sortable', null);
    const ariaLabel = computed(() => `Drag to reorder row ${(props.itemIndex ?? 0) + 1}`);

    function onPointerDown(e) {
      sortable?.startDrag?.(e, props.itemIndex);
    }

    function onKeyDown(e) {
      const move = sortable?.moveByKeyboard;
      if (!move) return;
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        move(-1, props.itemIndex);
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        move(+1, props.itemIndex);
      }
    }

    return () => h(DragOutlined, {
      class: 'sortable-icon',
      role: 'button',
      tabindex: 0,
      'aria-label': ariaLabel.value,
      onPointerdown: onPointerDown,
      onKeydown: onKeyDown,
    });
  },
});

export default defineComponent({
  name: 'TableSortable',
  inheritAttrs: false,
  props: {
    dataSource: { type: Array, default: () => [] },
    customRow: { type: Function, default: null },
    rowKey: { type: [String, Function], default: null },
    locale: {
      type: Object,
      default: () => ({ filterConfirm: 'OK', filterReset: 'Reset', emptyText: 'No data' }),
    },
  },
  emits: ['onsort'],
  setup(props, { emit, slots, attrs, expose }) {
    // null when idle; while dragging:
    //   { sourceIndex, targetIndex, pointerId, sourceKey }
    const drag = ref(null);
    const rootRef = ref(null);

    const isDragging = computed(() => drag.value !== null);

    // Resolve the row key for a record. Used to identify the source row
    // even after data-source is reordered live during drag.
    function keyOf(record, fallback) {
      const rk = props.rowKey;
      if (typeof rk === 'function') return rk(record);
      if (typeof rk === 'string') return record?.[rk];
      return fallback;
    }

    function attachListeners() {
      document.addEventListener('pointermove', onPointerMove, true);
      document.addEventListener('pointerup', onPointerUp, true);
      document.addEventListener('pointercancel', cancelDrag, true);
      document.addEventListener('keydown', cancelDrag, true);
    }

    function detachListeners() {
      document.removeEventListener('pointermove', onPointerMove, true);
      document.removeEventListener('pointerup', onPointerUp, true);
      document.removeEventListener('pointercancel', cancelDrag, true);
      document.removeEventListener('keydown', cancelDrag, true);
    }

    function startDrag(e, sourceIndex) {
      // Primary button only (mouse left / first touch).
      if (e.button != null && e.button !== 0) return;
      e.preventDefault();
      const record = props.dataSource?.[sourceIndex];
      drag.value = {
        sourceIndex,
        targetIndex: sourceIndex,
        pointerId: e.pointerId,
        sourceKey: keyOf(record, sourceIndex),
      };
      // Capture the pointer so move/up keep firing even if the cursor
      // leaves the icon. Try/catch — some older browsers throw on capture.
      if (e.target?.setPointerCapture && e.pointerId != null) {
        try { e.target.setPointerCapture(e.pointerId); } catch (_) { /* ignore */ }
      }
      attachListeners();
    }

    function onPointerMove(e) {
      const d = drag.value;
      if (!d) return;
      if (d.pointerId != null && e.pointerId !== d.pointerId) return;
      const root = rootRef.value;
      if (!root) return;
      const rows = root.querySelectorAll(`tr.${ROW_CLASS}`);
      if (!rows.length) return;
      const y = e.clientY;
      const firstRect = rows[0].getBoundingClientRect();
      const lastRect = rows[rows.length - 1].getBoundingClientRect();
      let target = d.targetIndex;
      if (y < firstRect.top) {
        target = 0;
      } else if (y > lastRect.bottom) {
        target = rows.length - 1;
      } else {
        for (let i = 0; i < rows.length; i++) {
          const rect = rows[i].getBoundingClientRect();
          if (y >= rect.top && y <= rect.bottom) {
            target = i;
            break;
          }
        }
      }
      if (target !== d.targetIndex) {
        drag.value = { ...d, targetIndex: target };
      }
    }

    function onPointerUp(e) {
      const d = drag.value;
      if (!d) return;
      if (d.pointerId != null && e.pointerId !== d.pointerId) return;
      detachListeners();
      const captured = d;
      drag.value = null;
      if (captured.sourceIndex !== captured.targetIndex) {
        emit('onsort', captured.sourceIndex, captured.targetIndex);
      }
    }

    function cancelDrag(e) {
      // Triggered by pointercancel and keydown. For keydown only act on
      // Escape; otherwise let the event propagate.
      if (e?.type === 'keydown' && e.key !== 'Escape') return;
      detachListeners();
      drag.value = null;
    }

    function moveByKeyboard(direction, sourceIndex) {
      const target = sourceIndex + direction;
      if (target < 0 || target >= (props.dataSource?.length ?? 0)) return;
      emit('onsort', sourceIndex, target);
    }

    function customRowRender(record, index) {
      const parent = typeof props.customRow === 'function' ? props.customRow(record, index) || {} : {};
      const d = drag.value;
      const isSource = d && keyOf(record, index) === d.sourceKey;
      // Vue 3 customRow shape: a flat object of attrs/listeners/class —
      // no nested props/on like Vue 2.
      return {
        ...parent,
        class: { [ROW_CLASS]: true, 'sortable-source-row': !!isSource, ...(parent.class || {}) },
      };
    }

    // Render-data: dataSource with the source row spliced into targetIndex.
    // When idle the original list is returned unchanged so a-table can
    // diff against a stable reference.
    const records = computed(() => {
      const d = drag.value;
      const src = props.dataSource ?? [];
      if (!d || d.sourceIndex === d.targetIndex) return src;
      const list = src.slice();
      const [item] = list.splice(d.sourceIndex, 1);
      list.splice(d.targetIndex, 0, item);
      return list;
    });

    expose({ startDrag, moveByKeyboard });

    return {
      rootRef, drag, isDragging, records, slots, attrs,
      startDrag, moveByKeyboard, customRowRender,
    };
  },
  // provide() needs to live at the options level so child components in
  // the rendered subtree resolve the same instance methods.
  provide() {
    return {
      sortable: {
        startDrag: (...a) => this.startDrag(...a),
        moveByKeyboard: (...a) => this.moveByKeyboard(...a),
      },
    };
  },
  beforeUnmount() {
    document.removeEventListener('pointermove', this.onPointerMove, true);
    document.removeEventListener('pointerup', this.onPointerUp, true);
    document.removeEventListener('pointercancel', this.cancelDrag, true);
    document.removeEventListener('keydown', this.cancelDrag, true);
  },
  render() {
    // Forward every passed slot to a-table by reusing the slot fn
    // directly. Vue 3 slots are scoped by default so no $scopedSlots dance.
    const tableSlots = {};
    for (const name of Object.keys(this.slots)) {
      tableSlots[name] = this.slots[name];
    }
    // Resolved at runtime so the user's app.use(Antd) registration wins;
    // avoids importing Table directly here.
    const ATable = resolveComponent('a-table');
    return h(
      'div',
      { ref: 'rootRef' },
      [h(
        ATable,
        {
          ...this.attrs,
          'data-source': this.records,
          'row-key': this.rowKey,
          customRow: this.customRowRender,
          locale: this.locale,
          class: ['sortable-table', { 'sortable-table-dragging': this.isDragging }],
        },
        tableSlots,
      )],
    );
  },
});
</script>

<style>
.sortable-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
  padding: 6px;
  border-radius: 6px;
  color: rgba(255, 255, 255, 0.5);
  transition: background-color 0.15s ease, color 0.15s ease;
  user-select: none;
  touch-action: none;
}

.sortable-icon:hover {
  color: rgba(255, 255, 255, 0.85);
  background: rgba(255, 255, 255, 0.06);
}

.sortable-icon:active {
  cursor: grabbing;
}

.sortable-icon:focus-visible {
  outline: 2px solid #008771;
  outline-offset: 2px;
}

.light .sortable-icon {
  color: rgba(0, 0, 0, 0.45);
}

.light .sortable-icon:hover {
  color: rgba(0, 0, 0, 0.85);
  background: rgba(0, 0, 0, 0.05);
}

.sortable-table-dragging .sortable-source-row>td {
  background: rgba(0, 135, 113, 0.10) !important;
  transition: background-color 0.18s ease;
}

.sortable-table-dragging .sortable-source-row .routing-index,
.sortable-table-dragging .sortable-source-row .outbound-index {
  opacity: 0.45;
}

.sortable-table-dragging .sortable-row>td {
  transition: background-color 0.18s ease;
}

.sortable-table-dragging,
.sortable-table-dragging * {
  user-select: none;
}
</style>
