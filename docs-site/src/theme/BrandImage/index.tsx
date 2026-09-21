import { useState } from 'react';

interface BrandImageProps {
  alt: string;
  dynamicSrc: string;
  fallbackSrc: string;
  className?: string;
}

export function BrandImage({
  alt,
  className,
  dynamicSrc,
  fallbackSrc,
}: BrandImageProps) {
  const [src, setSrc] = useState(dynamicSrc);

  return (
    <img
      src={src}
      alt={alt}
      className={className}
      onError={src === fallbackSrc ? undefined : () => setSrc(fallbackSrc)}
    />
  );
}
