-- Chi ha assegnato il programma.
--
-- NULL vuol dire "l'atleta se l'è scelto da solo" — dalla schermata dei piani o
-- dal generatore — ed è il caso normale: la colonna nasce tutta a NULL perché
-- prima del pannello non esisteva nessun altro modo di avere un programma.
--
-- Serve perché dal pannello un amministratore può sostituire il programma
-- attivo di un atleta, e senza questa colonna il risultato è indistinguibile da
-- una scelta dell'atleta stesso: chi si trova un piano diverso da quello di ieri
-- non avrebbe modo di sapere che gliel'ha cambiato l'allenatore, e l'allenatore
-- non avrebbe modo di ricordarsi di averlo fatto.
--
-- ON DELETE SET NULL e non RESTRICT: cancellare davvero un amministratore è
-- un'operazione da database, non dal pannello (lì si disattiva soltanto, e la
-- riga resta), ma se un giorno succede è meglio perdere il collegamento che
-- trovarsi bloccati da programmi vecchi di anni.
--
-- Ed è il motivo per cui l'indirizzo si scrive anche a parte, invece di
-- ricavarlo ogni volta dal join. Con il solo assigned_by, cancellare un
-- amministratore lo porterebbe a NULL, cioè esattamente al valore che significa
-- "scelto dall'atleta": un programma assegnato dall'allenatore comincerebbe a
-- risultare scelto da chi l'ha subito. Non un dato mancante — un dato falso, e
-- indistinguibile da quello vero. assigned_by_email conserva il fatto; assigned_by
-- conserva il collegamento finché la riga esiste.
ALTER TABLE user_programs
  ADD COLUMN IF NOT EXISTS assigned_by UUID REFERENCES admins(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS assigned_by_email TEXT;

-- Non serve una assigned_at: started_at è già il momento in cui il programma è
-- stato creato, e un programma assegnato viene creato nell'istante in cui lo si
-- assegna. Una seconda data direbbe la stessa cosa e prima o poi divergerebbe.
CREATE INDEX IF NOT EXISTS user_programs_assigned_by_idx
  ON user_programs (assigned_by) WHERE assigned_by IS NOT NULL;
