// @ant-design/icons render role="img" aria-label="<icon-name>", so screen readers speak
// "plus", "question-circle" etc. in English inside every button and label (s-ui#1293).
const MESSAGE =
  'Ant Design icon announces its English name; add aria-hidden if decorative, or aria-label with translated text.';

const EXPLICIT = new Set(['aria-hidden', 'aria-label', 'aria-labelledby']);

export default {
  meta: { name: 'antd-icon' },
  rules: {
    'explicit-a11y': {
      create(context) {
        const icons = new Set();
        return {
          ImportDeclaration(node) {
            if (node.source.value !== '@ant-design/icons') return;
            for (const spec of node.specifiers) {
              if (spec.type === 'ImportSpecifier') icons.add(spec.local.name);
            }
          },
          JSXOpeningElement(node) {
            if (node.name.type !== 'JSXIdentifier' || !icons.has(node.name.name)) return;
            const explicit = node.attributes.some(
              (attr) => attr.type !== 'JSXAttribute' || EXPLICIT.has(attr.name.name),
            );
            if (!explicit) context.report({ node, message: MESSAGE });
          },
        };
      },
    },
  },
};
