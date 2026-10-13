import type { HelpDocument } from '../../shared/api/types';
export const safeSectionId = (value: string) => /^[a-zA-Z0-9_-]{1,64}$/.test(value);
export function DocumentView({ document }: { document: HelpDocument }) {
  return (
    <article className="manual-content" aria-label="完整使用手册">
      {document.content.split(/\r?\n/).map((line, index) => {
        if (!line.trim()) return null;
        const heading = line.match(/^(#{1,3})\s+(.+)$/);
        if (!heading) return <p key={index}>{line}</p>;
        const explicit = heading[2].match(/\{#([a-zA-Z0-9_-]+)\}\s*$/)?.[1];
        const prefix = heading[2].match(/^([a-zA-Z0-9_-]+)\s*[:：]/)?.[1];
        const section = document.sections.find((item) => item.id === (explicit ?? prefix));
        const title = section?.title ?? heading[2].replace(/\s*\{#[^}]+\}\s*$/, '');
        return heading[1].length === 1 ? (
          <h2 key={index}>{title}</h2>
        ) : (
          <h3 key={index} id={section && safeSectionId(section.id) ? section.id : undefined}>
            {title}
          </h3>
        );
      })}
    </article>
  );
}
