import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useBlocker } from "@tanstack/react-router";

/** Protect persistent mapping edits without interrupting event-only lookups. */
export function RuleDraftGuard({
  dirty,
  onDiscard,
}: {
  dirty: boolean;
  onDiscard: () => void;
}) {
  const blocker = useBlocker({
    shouldBlockFn: () => dirty,
    enableBeforeUnload: dirty,
    withResolver: true,
  });
  return (
    <Modal
      opened={blocker.status === "blocked"}
      onClose={() => blocker.reset?.()}
      title="Discard unsaved rule changes?"
      centered
    >
      <Stack gap="md">
        <Text size="sm">
          Your local rule changes have not been saved. Keep editing to review
          them, or discard the draft and continue.
        </Text>
        <Group justify="end">
          <Button onClick={() => blocker.reset?.()}>Keep editing</Button>
          <Button
            color="red"
            onClick={() => {
              onDiscard();
              blocker.proceed?.();
            }}
          >
            Discard and continue
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
