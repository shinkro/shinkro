import { useEffect } from "react";
import { Flex, Group, Loader, Stack, Text, Button, Paper, Image } from "@mantine/core";
import { FaArrowRightArrowLeft } from "react-icons/fa6";
import { SiAnilist } from "react-icons/si";
import Logo from "@app/logo.svg";

export const AnilistAuthCallback = () => {
    useEffect(() => {
        // AniList callback is handled server-side via GET /api/anilistauth/callback
        // This page just notifies the opener and closes itself
        window.opener?.postMessage({ type: "anilist-auth" }, window.location.origin);
    }, []);

    return (
        <Flex
            direction={"column"}
            w={"100%"}
            maw={"600px"}
            miw={"280px"}
            mx={"auto"}
            pt={"10vh"}
            align={"stretch"}
        >
            <Paper withBorder p="md" shadow="xl">
                <Group justify={"center"}>
                    <Image src={Logo} fit="contain" h={80} />
                    <FaArrowRightArrowLeft size={50} />
                    <SiAnilist size={80} color={"#02a9ff"} />
                </Group>
                <Stack align="center" mt="md">
                    <Loader size="xl" />
                    <Text>Authenticating with AniList...</Text>
                    <Text c={"dimmed"}>You may close this window now.</Text>
                    <Button onClick={() => window.close()}>CLOSE WINDOW</Button>
                </Stack>
            </Paper>
        </Flex>
    );
};
