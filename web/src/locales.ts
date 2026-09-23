export type Locale = {
  local_id: number;
  name: string;
  category: string;
  active: boolean;
};
export type AeroZone = {
  zone_id: number;
  local_id: number;
  floor_id: number;
  name: string;
  zone_type: string;
  color: string;
  geometry: GeoJSON.Polygon;
  area_m2: number;
  active: boolean;
};

async function unwrap<T>(r: Response): Promise<T> {
  if (!r.ok) throw Error((await r.json()).error ?? "Error de red");
  return r.json();
}

export async function loadLocales(): Promise<Locale[]> {
  return unwrap(await fetch("/api/v1/locales"));
}
export async function patchLocale(
  id: number,
  patch: Partial<Pick<Locale, "name" | "category">>,
): Promise<Locale> {
  return unwrap(
    await fetch(`/api/v1/locales/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    }),
  );
}
export async function loadAeroZones(): Promise<AeroZone[]> {
  return unwrap(await fetch("/api/v1/aero-zones?floor_id=3"));
}
export async function createAeroZone(input: {
  local_id: number;
  name: string;
  zone_type: string;
  geometry: GeoJSON.Polygon;
}): Promise<AeroZone> {
  return unwrap(
    await fetch("/api/v1/aero-zones", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    }),
  );
}
export async function patchAeroZone(
  id: number,
  patch: Partial<Pick<AeroZone, "name" | "zone_type" | "geometry">>,
): Promise<AeroZone> {
  return unwrap(
    await fetch(`/api/v1/aero-zones/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    }),
  );
}
