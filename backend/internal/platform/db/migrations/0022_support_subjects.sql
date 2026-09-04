-- Tiketa e mbështetjes mund t'i referohet edhe një porosie, pakoje ose kërkese për mjeshtër.
--
-- Deri tani njihte vetëm `ride_id`, sepse kur u shkrua ekzistonin vetëm udhëtimet. Rrjedhoja te
-- aplikacioni ishte se ekranet e ndjekjes së porosisë, pakos dhe shërbimit nuk kishin asnjë buton
-- ndihme: një ankesë nuk kishte ku ta thoshte për çfarë bëhej fjalë.
--
-- Tri kolona me çelës të huaj e jo një çift (lloj, id): çelësi i huaj e mban lidhjen të vërtetë,
-- dhe një tiketë që tregon një porosi të fshirë do të ishte një rresht që nuk hapet dot.

ALTER TABLE support_tickets
  ADD COLUMN order_id          uuid REFERENCES orders(id) ON DELETE SET NULL,
  ADD COLUMN parcel_id         uuid REFERENCES parcels(id) ON DELETE SET NULL,
  ADD COLUMN service_request_id uuid REFERENCES service_requests(id) ON DELETE SET NULL;

-- Një tiketë flet për një gjë të vetme. Pa këtë, agjenti do të shihte dy referenca dhe nuk do të
-- dinte cilën të hapte.
ALTER TABLE support_tickets
  ADD CONSTRAINT support_tickets_one_subject CHECK (
    (CASE WHEN ride_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN order_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN parcel_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN service_request_id IS NULL THEN 0 ELSE 1 END) <= 1
  );

-- E njëjta gjë për raportet e sigurisë: një problem gjatë një dërgese duhet të dijë cilën dërgesë.
ALTER TABLE safety_reports
  ADD COLUMN order_id  uuid REFERENCES orders(id) ON DELETE SET NULL,
  ADD COLUMN parcel_id uuid REFERENCES parcels(id) ON DELETE SET NULL;
