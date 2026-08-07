-- Ritiro di una template dal catalogo.
--
-- Serve perché cancellarla spesso non si può: user_programs.template_id la
-- referenzia senza ON DELETE, quindi finché un solo atleta ha in corso — o ha
-- avuto — un programma nato da quella template, la DELETE fallisce. Ed è giusto
-- così: il programma dell'atleta è una copia, ma il collegamento a com'era nata
-- è l'unica cosa che permette di sapere da dove viene.
--
-- Archiviare risolve il caso vero, che non è "questa template non è mai
-- esistita" ma "non voglio più assegnarla a nessuno". Le sparisce dal catalogo
-- (vedi GetTemplates) e dal menù di assegnazione, mentre i programmi già in
-- corso continuano a funzionare: sono copie, non leggono più nulla da qui.
ALTER TABLE plan_templates
  ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
