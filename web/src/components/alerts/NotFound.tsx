import {Link} from "@tanstack/react-router";
import {ExternalLink} from "@components/ExternalLink";
import Logo from "@app/logo.svg";
import {Text, Button, Center, Stack, Container, Image, Paper} from "@mantine/core";

export const NotFound = () => {
    return (
        <div style={{padding: '1rem'}}>
            <Center pt="10vh">
                <Container size="md">
                    <Paper withBorder radius="md" shadow="xl" p="xl">
                        <Image src={Logo} alt="Logo" width={120} height={120} fit="contain"/>
                        <Stack align="center" mt="md" gap="xs">
                            <Text size="xl" fw={700}>
                                404 — Lost in the Plot
                            </Text>
                            <Text size="lg" c="dimmed" ta="center">
                                You’ve wandered too far and triggered a secret route...
                            </Text>
                            <Text size="md" c="dimmed" ta="center">
                                If you believe this was not fate but a bug,
                            </Text>
                            <Text size="md" c="dimmed" ta="center">
                                send a raven to our{" "}
                                <ExternalLink href="https://github.com/shinkro/shinkro">
                                    GitHub guild
                                </ExternalLink>{" "}
                                or rally at{" "}
                                <ExternalLink href="https://discord.gg/ZkYdfNgbAT">
                                    our Discord tavern
                                </ExternalLink>
                                .
                            </Text>
                            <Text size="md" c="dimmed" ta="center">
                                Otherwise, ready your gear — we’ll guide you back to the main story!
                            </Text>
                            <Link to="/">
                                <Button size="md" color="blue" mt="md">
                                    Return to Dashboard
                                </Button>
                            </Link>
                        </Stack>
                    </Paper>
                </Container>
            </Center>
        </div>
    );
};
