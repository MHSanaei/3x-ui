// jsx-a11y cannot see through Ant Design components, so a Switch or Select with no
// tie to its visible label reaches screen readers unnamed (s-ui#1293). Guard it here.
const CONTROLS = new Set([
  'Input',
  'Input.Password',
  'Input.TextArea',
  'Input.Search',
  'InputNumber',
  'Select',
  'AutoComplete',
  'Switch',
  'DatePicker',
  'DateTimePicker',
  'DatePicker.RangePicker',
  'TimePicker',
  'Cascader',
  'TreeSelect',
]);

const UNNAMED =
  'Form control has no accessible name; give it aria-label, or an id wired to a visible label.';
const NESTED =
  'Form.Item name hands its id only to its direct child; wire htmlFor/id or aria-label for this nested control.';
const UNTIED =
  'Form.Item label without name is not tied to this control; pass htmlFor on the Form.Item and the same id on the control.';

function tagName(node) {
  if (!node) return '';
  if (node.type === 'JSXIdentifier') return node.name;
  if (node.type === 'JSXMemberExpression') return `${tagName(node.object)}.${node.property.name}`;
  return '';
}

function attrs(element) {
  const map = new Map();
  let spread = false;
  for (const attr of element.openingElement.attributes ?? []) {
    if (attr.type === 'JSXAttribute') map.set(attr.name.name, attr);
    else spread = true;
  }
  return { map, spread };
}

// `direct` marks the innermost element: the only one whose cloneElement reaches the control.
function* labellingAncestors(node) {
  let direct = true;
  for (let cur = node.parent; cur; cur = cur.parent) {
    if (cur.type !== 'JSXElement') continue;
    yield { element: cur, direct };
    direct = false;
  }
}

function check(node) {
  const { map, spread } = attrs(node);
  if (map.has('aria-label') || map.has('aria-labelledby') || spread) return null;
  const hasId = map.has('id');
  for (const { element, direct } of labellingAncestors(node)) {
    const name = tagName(element.openingElement.name);
    const parentAttrs = attrs(element).map;
    if (name === 'label') return null;
    if (name === 'FormField' && parentAttrs.has('label')) return null;
    if (name === 'SettingListItem' && parentAttrs.has('title')) return direct ? null : UNNAMED;
    if (name === 'Form.Item' && parentAttrs.has('label')) {
      if (parentAttrs.has('name')) return direct ? null : NESTED;
      if (parentAttrs.has('htmlFor')) return hasId ? null : UNTIED;
      return UNTIED;
    }
  }
  return hasId ? null : UNNAMED;
}

export default {
  meta: { name: 'control-label' },
  rules: {
    'has-name': {
      create(context) {
        return {
          JSXElement(node) {
            if (!CONTROLS.has(tagName(node.openingElement.name))) return;
            const message = check(node);
            if (message) context.report({ node: node.openingElement, message });
          },
        };
      },
    },
  },
};
