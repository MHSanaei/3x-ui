import gc

import weakref
import sys
import greenlet


from . import TestCase
from .leakcheck import fails_leakcheck_on_py314_or_less
# These only work with greenlet gc support
# which is no longer optional.
assert greenlet.GREENLET_USE_GC

class TestGC(TestCase):
    def test_dead_circular_ref(self):
        o = weakref.ref(greenlet.greenlet(greenlet.getcurrent).switch())
        gc.collect()
        if o() is not None:
            print("O IS NOT NONE.", sys.getrefcount(o()))
        self.assertIsNone(o())
        self.assertFalse(gc.garbage, gc.garbage)

    def test_circular_greenlet(self):
        class circular_greenlet(greenlet.greenlet):
            self = None
        o = circular_greenlet()
        o.self = o
        o = weakref.ref(o)
        gc.collect()
        self.assertIsNone(o())
        self.assertFalse(gc.garbage, gc.garbage)

    def test_inactive_ref(self):
        class inactive_greenlet(greenlet.greenlet):
            def __init__(self):
                greenlet.greenlet.__init__(self, run=self.run)

            def run(self):
                pass
        o = inactive_greenlet()
        o = weakref.ref(o)
        gc.collect()
        self.assertIsNone(o())
        self.assertFalse(gc.garbage, gc.garbage)

    @fails_leakcheck_on_py314_or_less
    def test_finalizer_crash(self):
        # This test is designed to crash when active greenlets
        # are made garbage collectable, until the underlying
        # problem is resolved. How does it work:
        # - order of object creation is important
        # - array is created first, so it is moved to unreachable first
        # - we create a cycle between a greenlet and this array
        # - we create an object that participates in gc, is only
        #   referenced by a greenlet, and would corrupt gc lists
        #   on destruction, the easiest is to use an object with
        #   a finalizer
        # - because array is the first object in unreachable it is
        #   cleared first, which causes all references to greenlet
        #   to disappear and causes greenlet to be destroyed, but since
        #   it is still live it causes a switch during gc, which causes
        #   an object with finalizer to be destroyed, which causes stack
        #   corruption and then a crash

        class object_with_finalizer(object):
            def __del__(self):
                pass
        array = []
        parent = greenlet.getcurrent()
        def greenlet_body():
            greenlet.getcurrent().object = object_with_finalizer()
            try:
                parent.switch()
            except greenlet.GreenletExit:
                print("Got greenlet exit!")
            finally:
                del greenlet.getcurrent().object
        g = greenlet.greenlet(greenlet_body)
        g.array = array
        array.append(g)
        g.switch()
        del array
        del g
        greenlet.getcurrent()
        gc.collect()

    def test_crashing_deferred_object(self):
        if sys.version_info < (3, 15):
            self.skipTest("Test is 3.15+ only")
        import doctest
        def with_doctest():
            """
            >>> import gc
            >>> from greenlet import getcurrent, greenlet, GreenletExit
            >>> def outer():
            ...     gc.collect()
            >>> outer_glet = greenlet(outer)
            >>> outer_glet.switch()
            """
        doctest.run_docstring_examples(with_doctest, dict())

    def test_cycle_in_suspended_frame(self):
        if sys.version_info < (3, 15):
            self.skipTest("Test is 3.15+ only")
        import doctest
        def with_doctest():
            """
            >>> import gc
            >>> from greenlet import getcurrent, greenlet
            >>> class Cycle:
            ...     def __del__(self):
            ...         print("(Running finalizer)")
            >>> def collect_it():
            ...     print("Collecting garbage")
            ...     gc.collect()
            >>> def inner():
            ...     cycle1 = Cycle()
            ...     cycle2 = Cycle()
            ...     cycle1.cycle = cycle2
            ...     cycle2.cycle = cycle1
            ...     getcurrent().parent.switch()
            >>> def outer():
            ...     glet = greenlet(inner)
            ...     glet.switch()
            ...     collect_it()

            >>> outer_glet = greenlet(outer)
            >>> outer_glet.switch()
            Collecting garbage
            >>> outer_glet.dead
            True
            >>> collect_it()
            Collecting garbage
            (Running finalizer)
            (Running finalizer)
            """
        doctest.run_docstring_examples(with_doctest, dict())
