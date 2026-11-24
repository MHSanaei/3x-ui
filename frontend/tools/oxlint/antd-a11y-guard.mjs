// Ant Design Spin wraps its content in aria-live="polite", so every live stat inside is
// read aloud; Select tags hide their remove "x" from screen readers and keyboards.
const SPIN_LIVE =
  'Spin wraps its content in aria-live="polite", so every update inside is read aloud; pass aria-live="off".';
const NO_TAG_RENDER =
  'Select tag remove buttons are aria-hidden and unfocusable; pass tagRender={selectTagRender}.';
const HIDDEN_TAGS =
  'maxTagCount hides selected values behind "+N", out of reach of screen readers; show every tag.';

function attrs(node) {
  const map = new Map();
  let spread = false;
  for (const attr of node.attributes) {
    if (attr.type === 'JSXAttribute') map.set(attr.name.name, attr);
    else spread = true;
  }
  return { map, spread };
}

function hasContent(element) {
  return element.children.some((c) => c.type !== 'JSXText' || c.value.trim() !== '');
}

export default {
  meta: { name: 'antd-a11y' },
  rules: {
    'spin-content-not-live': {
      create(context) {
        const spins = new Set();
        return {
          ImportDeclaration(node) {
            if (node.source.value !== 'antd') return;
            for (const spec of node.specifiers) {
              if (spec.type === 'ImportSpecifier' && spec.imported.name === 'Spin') {
                spins.add(spec.local.name);
              }
            }
          },
          JSXElement(node) {
            const opening = node.openingElement;
            if (opening.name.type !== 'JSXIdentifier' || !spins.has(opening.name.name)) return;
            if (!hasContent(node)) return;
            const { map, spread } = attrs(opening);
            if (!spread && !map.has('aria-live'))
              context.report({ node: opening, message: SPIN_LIVE });
          },
        };
      },
    },
    'select-tag-remove': {
      create(context) {
        const selects = new Set();
        return {
          ImportDeclaration(node) {
            if (node.source.value !== 'antd') return;
            for (const spec of node.specifiers) {
              if (spec.type === 'ImportSpecifier' && spec.imported.name === 'Select') {
                selects.add(spec.local.name);
              }
            }
          },
          JSXOpeningElement(node) {
            if (node.name.type !== 'JSXIdentifier' || !selects.has(node.name.name)) return;
            const { map, spread } = attrs(node);
            if (!map.has('mode')) return;
            if (map.has('maxTagCount')) context.report({ node, message: HIDDEN_TAGS });
            else if (!spread && !map.has('tagRender'))
              context.report({ node, message: NO_TAG_RENDER });
          },
        };
      },
    },
  },
};
