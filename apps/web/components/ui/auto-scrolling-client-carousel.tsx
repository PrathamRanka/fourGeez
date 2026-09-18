import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export type ClientCarouselItem = {
  name: string;
  mark?: ReactNode;
};

type AutoScrollingClientCarouselProps = {
  className?: string;
  clients: readonly ClientCarouselItem[];
};

type RibbonProps = {
  clients: readonly ClientCarouselItem[];
  reverse?: boolean;
};

// Ribbon adapts the MIT Ruixen continuous client strip with a seamless three-copy loop.
function Ribbon({ clients, reverse = false }: RibbonProps) {
  const repeatedClients = [...clients, ...clients, ...clients];

  return (
    <div className="client-ribbon-viewport" aria-hidden="true">
      <div
        className={cn(
          "client-ribbon-track",
          reverse && "client-ribbon-track-reverse",
        )}
      >
        {repeatedClients.map((client, index) => (
          <span className="client-ribbon-item" key={`${client.name}-${index}`}>
            {client.mark}
            {client.name}
          </span>
        ))}
      </div>
    </div>
  );
}

// AutoScrollingClientCarousel presents supported stacks without implying customer endorsements.
export function AutoScrollingClientCarousel({
  className,
  clients,
}: AutoScrollingClientCarouselProps) {
  return (
    <div className={cn("client-ribbons", className)}>
      <Ribbon clients={clients} />
      <Ribbon clients={[...clients].reverse()} reverse />
    </div>
  );
}
