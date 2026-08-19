// Ports the no-restricted-syntax selectors of the old ESLint config: oxlint has
// no no-restricted-syntax, so the #6121/#6127 cleared-InputNumber guard lives here.
const MESSAGE =
  'A cleared InputNumber must not write a synthetic value; wrap the handler with onNumber() from @/utils/onNumber (see #6127).';

const isLiteral = (node) =>
  !!node && /^(Literal|NumericLiteral|StringLiteral|BooleanLiteral)$/.test(node.type);

function walk(node, visit) {
  if (!node || typeof node !== 'object') return;
  if (Array.isArray(node)) {
    for (const child of node) walk(child, visit);
    return;
  }
  if (typeof node.type === 'string') visit(node);
  for (const key of Object.keys(node)) {
    if (key !== 'parent') walk(node[key], visit);
  }
}

// `Number(v) || N`, `typeof v === 'number' ? v : N` and `v ?? N` all turn a
// cleared field into a stored N.
function isSyntheticClear(node) {
  if (node.type === 'LogicalExpression' && node.operator === '||') {
    return [node.left, node.right].some(
      (side) => side?.type === 'CallExpression' && side.callee?.name === 'Number',
    );
  }
  if (node.type === 'ConditionalExpression') {
    return node.test?.left?.operator === 'typeof' && isLiteral(node.alternate);
  }
  if (node.type === 'LogicalExpression' && node.operator === '??') {
    return isLiteral(node.right);
  }
  return false;
}

export default {
  meta: { name: 'input-number' },
  rules: {
    'no-synthetic-clear': {
      create(context) {
        return {
          JSXElement(node) {
            if (node.openingElement?.name?.name !== 'InputNumber') return;
            for (const attr of node.openingElement.attributes ?? []) {
              if (attr.type !== 'JSXAttribute' || attr.name?.name !== 'onChange') continue;
              walk(attr.value, (inner) => {
                if (isSyntheticClear(inner)) context.report({ node: inner, message: MESSAGE });
              });
            }
          },
        };
      },
    },
  },
};
