import defaultMdxComponents from 'fumadocs-ui/mdx';
import { Tab, Tabs } from 'fumadocs-ui/components/tabs';
import { Step, Steps } from 'fumadocs-ui/components/steps';
import { Mermaid } from '@/components/mdx/mermaid';
import { RealityConfigGenerator } from '@/components/tools/reality-config-generator';
import { ShareLinkInspector } from '@/components/tools/share-link-inspector';
import { InstallCommandBuilder } from '@/components/tools/install-command-builder';
import { ReverseProxyGenerator } from '@/components/tools/reverse-proxy-generator';
import { ProtocolWizard } from '@/components/tools/protocol-wizard';
import { FirewallRulesGenerator } from '@/components/tools/firewall-rules-generator';
import { OutboundGenerator } from '@/components/tools/outbound-generator';
import { RoutingBuilder } from '@/components/tools/routing-builder';
import { SubscriptionBuilder } from '@/components/tools/subscription-builder';
import { TelegramSetupHelper } from '@/components/tools/telegram-setup-helper';
import { ApiRequestBuilder } from '@/components/tools/api-request-builder';
import { OpenAPIPage } from '@/components/openapi-page';
import type { MDXComponents } from 'mdx/types';

export function getMDXComponents(components?: MDXComponents) {
  return {
    ...defaultMdxComponents,
    Tab,
    Tabs,
    Step,
    Steps,
    Mermaid,
    RealityConfigGenerator,
    ShareLinkInspector,
    InstallCommandBuilder,
    ReverseProxyGenerator,
    ProtocolWizard,
    FirewallRulesGenerator,
    OutboundGenerator,
    RoutingBuilder,
    SubscriptionBuilder,
    TelegramSetupHelper,
    ApiRequestBuilder,
    OpenAPIPage,
    ...components,
  } satisfies MDXComponents;
}

export const useMDXComponents = getMDXComponents;

declare global {
  type MDXProvidedComponents = ReturnType<typeof getMDXComponents>;
}
