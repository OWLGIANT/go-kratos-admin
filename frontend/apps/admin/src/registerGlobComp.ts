import type { App } from 'vue';
import VueUeditorWrap from 'vue-ueditor-wrap';

import {
  Button,
  Card,
  Divider,
  Dropdown,
  Form,
  Input,
  Layout,
  Menu,
  Modal,
  PageHeader,
  Popconfirm,
  Radio,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tabs,
  Tag,
  Tree,
} from 'ant-design-vue';

/**
 * 注册全局组件
 * @param app
 */
export function registerGlobComp(app: App) {
  app
    .use(Input)
    .use(Button)
    .use(Layout)
    .use(Space)
    .use(Card)
    .use(Switch)
    .use(Popconfirm)
    .use(Dropdown)
    .use(Tag)
    .use(Tabs)
    .use(Divider)
    .use(Menu)
    .use(Table)
    .use(Form)
    .use(Select)
    .use(Radio)
    .use(Modal)
    .use(Spin)
    .use(PageHeader)
    .use(VueUeditorWrap)
    .use(Tree);
}
