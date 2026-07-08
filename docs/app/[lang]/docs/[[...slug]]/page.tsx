import { getPageImage, getPageMarkdownUrl, source } from '@/lib/source';
import {
  DocsBody,
  DocsDescription,
  DocsPage,
  DocsTitle,
  MarkdownCopyButton,
  ViewOptionsPopover,
} from 'fumadocs-ui/layouts/docs/page';
import { notFound } from 'next/navigation';
import { getMDXComponents } from '@/components/mdx';
import { OpenAPIPage as BaseOpenAPIPage } from '@/components/openapi-page';
import { openapi } from '@/lib/openapi';
import type { Metadata } from 'next';
import type { ComponentProps } from 'react';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import { gitConfig } from '@/lib/shared';

export default async function Page(props: PageProps<'/[lang]/docs/[[...slug]]'>) {
  const { lang, slug } = await props.params;
  const page = source.getPage(slug, lang);
  if (!page) notFound();

  const MDX = page.data.body;
  const markdownUrl = getPageMarkdownUrl(page).url;
  const editUrl = `https://github.com/${gitConfig.user}/${gitConfig.repo}/blob/${gitConfig.branch}/${gitConfig.docsDir}/${page.path}`;

  // Generated API reference pages carry `_openapi` metadata. Preload the spec
  // on the server (highlighting included) so the client OpenAPIPage doesn't have
  // to load it at render time.
  const isOpenAPI = Boolean((page.data as { _openapi?: unknown })._openapi);
  const extraComponents: Record<string, unknown> = {};
  if (isOpenAPI) {
    const preloaded = await openapi.preloadOpenAPIPage(page);
    function PreloadedOpenAPIPage(p: ComponentProps<typeof BaseOpenAPIPage>) {
      return <BaseOpenAPIPage {...p} {...preloaded} />;
    }
    extraComponents.OpenAPIPage = PreloadedOpenAPIPage;
  }

  return (
    <DocsPage toc={page.data.toc} full={page.data.full}>
      <DocsTitle>{page.data.title}</DocsTitle>
      <DocsDescription className="mb-0">{page.data.description}</DocsDescription>
      <div className="flex flex-row items-center gap-2 border-b pb-6">
        <MarkdownCopyButton markdownUrl={markdownUrl} />
        <ViewOptionsPopover markdownUrl={markdownUrl} githubUrl={editUrl} />
      </div>
      <DocsBody>
        <MDX
          components={getMDXComponents({
            // allows linking to other pages with relative file paths
            a: createRelativeLink(source, page),
            ...extraComponents,
          })}
        />
      </DocsBody>
    </DocsPage>
  );
}

export async function generateStaticParams() {
  return source.generateParams();
}

export async function generateMetadata(
  props: PageProps<'/[lang]/docs/[[...slug]]'>,
): Promise<Metadata> {
  const { lang, slug } = await props.params;
  const page = source.getPage(slug, lang);
  if (!page) notFound();

  return {
    title: page.data.title,
    description: page.data.description,
    openGraph: {
      images: getPageImage(page).url,
    },
  };
}
